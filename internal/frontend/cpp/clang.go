package cpp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os/exec"
	"strconv"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

const (
	// libc++-heavy translation units routinely produce 70-100 MiB JSON ASTs.
	// The limit remains explicit and fail-closed while admitting the frozen
	// real C++ gate without truncating compiler evidence.
	maxASTBytes        = 256 << 20
	maxDiagnosticBytes = 1 << 20
)

var errOutputLimit = errors.New("clang output exceeds frontend limit")

// astNode contains the stable subset of Clang's JSON AST consumed by the
// frontend. Unknown JSON fields are deliberately ignored; unknown AST node
// kinds in user code are not.
type astNode struct {
	ID                   string        `json:"id"`
	ParentDeclContextID  string        `json:"parentDeclContextId"`
	Kind                 string        `json:"kind"`
	Name                 string        `json:"name"`
	MangledName          string        `json:"mangledName"`
	PreviousDecl         string        `json:"previousDecl"`
	ReferencedMemberDecl string        `json:"referencedMemberDecl"`
	Opcode               string        `json:"opcode"`
	Value                astScalar     `json:"value"`
	ValueCategory        string        `json:"valueCategory"`
	CastKind             string        `json:"castKind"`
	IsImplicit           bool          `json:"isImplicit"`
	Implicit             bool          `json:"implicit"`
	IsReferenced         bool          `json:"isReferenced"`
	IsArrow              bool          `json:"isArrow"`
	IsPostfix            bool          `json:"isPostfix"`
	CanOverflow          bool          `json:"canOverflow"`
	HasElse              bool          `json:"hasElse"`
	Type                 astType       `json:"type"`
	Loc                  astLocation   `json:"loc"`
	Range                astRange      `json:"range"`
	ReferencedDecl       astReferenced `json:"referencedDecl"`
	Inner                []*astNode    `json:"inner"`
}

// Clang encodes literal values inconsistently across node kinds (JSON strings
// for integers/strings and JSON booleans for CXXBoolLiteralExpr). astScalar
// normalizes those representations without accepting objects or arrays.
type astScalar string

func (v *astScalar) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*v = astScalar(text)
		return nil
	}
	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		*v = astScalar(strconv.FormatBool(boolean))
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*v = astScalar(number.String())
		return nil
	}
	return fmt.Errorf("unsupported clang scalar %s", data)
}

type astType struct {
	QualType string `json:"qualType"`
}

type astReferenced struct {
	ID   string  `json:"id"`
	Kind string  `json:"kind"`
	Name string  `json:"name"`
	Type astType `json:"type"`
}

type astRange struct {
	Begin astLocation `json:"begin"`
	End   astLocation `json:"end"`
}

type astLocation struct {
	File         string       `json:"file"`
	Offset       int          `json:"offset"`
	Line         int          `json:"line"`
	Col          int          `json:"col"`
	TokLen       int          `json:"tokLen"`
	SpellingLoc  *astLocation `json:"spellingLoc"`
	ExpansionLoc *astLocation `json:"expansionLoc"`
}

func (l astLocation) expansion() astLocation {
	if l.ExpansionLoc != nil {
		return l.ExpansionLoc.expansion()
	}
	return l
}

func (l astLocation) spelling() astLocation {
	if l.SpellingLoc != nil {
		return l.SpellingLoc.spelling()
	}
	return l
}

func (n *astNode) sourceRange() astRange {
	if n == nil {
		return astRange{}
	}
	return astRange{
		Begin: n.Range.Begin.expansion(),
		End:   n.Range.End.expansion(),
	}
}

type clangResult struct {
	Root          *astNode
	Executable    string
	Version       string
	IntegerWidths map[string]int
	LLVMIR        string
	Stderr        string
}

func clangAST(ctx context.Context, workspace semanticir.WorkspaceRef, executable, directory, sourcePath string, compileFlags, filters []string) (clangResult, error) {
	root := astNode{Kind: "TranslationUnitDecl"}
	var combinedStderr strings.Builder
	if len(filters) == 0 {
		return clangResult{Executable: executable}, fmt.Errorf("no exact Clang AST declaration filters were derived from the proof scope")
	}
	for _, filter := range filters {
		args := []string{"-x", "c++"}
		args = append(args, compileFlags...)
		args = append(args, "-fsyntax-only", "-fno-color-diagnostics", "-Xclang", "-ast-dump=json", "-Xclang", "-ast-dump-filter="+filter, sourcePath)
		stdout := &limitedBuffer{limit: maxASTBytes}
		stderr := &limitedBuffer{limit: maxDiagnosticBytes}
		cmd := exec.CommandContext(ctx, executable, args...)
		configureCPPCommand(cmd, workspace)
		cmd.Dir = directory
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		if errors.Is(stdout.err, errOutputLimit) {
			return clangResult{Executable: executable, Stderr: stderr.String()}, fmt.Errorf("parse C++ filter %q: %w (%d bytes)", filter, errOutputLimit, maxASTBytes)
		}
		if err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = err.Error()
			}
			return clangResult{Executable: executable, Stderr: detail}, fmt.Errorf("clang rejected C++ source for filter %q: %s", filter, detail)
		}
		combinedStderr.WriteString(stderr.String())
		dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
		decoded := 0
		for {
			var declaration astNode
			if err := dec.Decode(&declaration); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return clangResult{Executable: executable, Stderr: stderr.String()}, fmt.Errorf("decode Clang JSON AST filter %q: %w", filter, err)
			}
			root.Inner = append(root.Inner, &declaration)
			decoded++
		}
		if decoded == 0 {
			return clangResult{Executable: executable, Stderr: stderr.String()}, fmt.Errorf("Clang AST filter %q matched no declaration", filter)
		}
	}

	version := clangVersion(ctx, workspace, executable)
	widths, err := clangIntegerWidths(ctx, workspace, executable, directory, compileFlags)
	if err != nil {
		return clangResult{Root: &root, Executable: executable, Version: version, Stderr: strings.TrimSpace(combinedStderr.String())}, err
	}
	llvmIR, err := clangLLVMIR(ctx, workspace, executable, directory, sourcePath, compileFlags)
	if err != nil {
		return clangResult{Root: &root, Executable: executable, Version: version, IntegerWidths: widths, Stderr: strings.TrimSpace(combinedStderr.String())}, err
	}
	return clangResult{Root: &root, Executable: executable, Version: version, IntegerWidths: widths, LLVMIR: llvmIR, Stderr: strings.TrimSpace(combinedStderr.String())}, nil
}

// clangLLVMIR compiles the exact frozen focus file with the normalized
// compilation-database argument vector. The only added arguments select
// textual LLVM output instead of the compile command's object output.
func clangLLVMIR(ctx context.Context, workspace semanticir.WorkspaceRef, executable, directory, sourcePath string, compileFlags []string) (string, error) {
	args := []string{"-x", "c++"}
	args = append(args, compileFlags...)
	args = append(args, "-S", "-emit-llvm", "-fno-color-diagnostics", "-o", "-", sourcePath)
	stdout := &limitedBuffer{limit: maxASTBytes}
	stderr := &limitedBuffer{limit: maxDiagnosticBytes}
	cmd := exec.CommandContext(ctx, executable, args...)
	configureCPPCommand(cmd, workspace)
	cmd.Dir = directory
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("emit pinned Clang LLVM IR: %s", detail)
	}
	if errors.Is(stdout.err, errOutputLimit) {
		return "", fmt.Errorf("emit pinned Clang LLVM IR: %w (%d bytes)", errOutputLimit, maxASTBytes)
	}
	if len(bytes.TrimSpace(stderr.Bytes())) != 0 {
		return "", fmt.Errorf("pinned Clang LLVM emission produced diagnostics: %s", strings.TrimSpace(stderr.String()))
	}
	ir := stdout.String()
	if !strings.Contains(ir, "target triple") || !strings.Contains(ir, "define ") {
		return "", fmt.Errorf("pinned Clang emitted incomplete LLVM IR")
	}
	return ir, nil
}

func clangIntegerWidths(ctx context.Context, workspace semanticir.WorkspaceRef, executable, directory string, compileFlags []string) (map[string]int, error) {
	args := []string{"-x", "c++"}
	args = append(args, compileFlags...)
	args = append(args, "-dM", "-E", "-")
	stdout := &limitedBuffer{limit: maxDiagnosticBytes}
	stderr := &limitedBuffer{limit: maxDiagnosticBytes}
	cmd := exec.CommandContext(ctx, executable, args...)
	configureCPPCommand(cmd, workspace)
	cmd.Dir = directory
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("query pinned clang integer widths: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	macroNames := map[string]string{
		"__SCHAR_WIDTH__":     "signed char",
		"__SCHAR_MAX__":       "signed char",
		"__SHRT_WIDTH__":      "short",
		"__INT_WIDTH__":       "int",
		"__LONG_WIDTH__":      "long",
		"__LONG_LONG_WIDTH__": "long long",
		"__LLONG_WIDTH__":     "long long",
	}
	widths := make(map[string]int, len(macroNames))
	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "#define" {
			continue
		}
		name, wanted := macroNames[fields[1]]
		if !wanted {
			continue
		}
		width, err := strconv.Atoi(fields[2])
		if fields[1] == "__SCHAR_MAX__" {
			maximum, parseErr := strconv.ParseUint(strings.TrimRight(fields[2], "uUlL"), 0, 64)
			if parseErr == nil {
				width, err = bits.Len64(maximum)+1, nil
			}
		}
		if err == nil && width > 0 && width <= 64 {
			widths[name] = width
		}
	}
	for _, name := range macroNames {
		if widths[name] == 0 {
			return nil, fmt.Errorf("pinned clang did not report %s", name)
		}
	}
	return widths, nil
}

func clangVersion(ctx context.Context, workspace semanticir.WorkspaceRef, executable string) string {
	cmd := exec.CommandContext(ctx, executable, "--version")
	configureCPPCommand(cmd, workspace)
	var out limitedBuffer
	out.limit = 16 << 10
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "unknown"
	}
	version := strings.TrimSpace(out.String())
	if version == "" {
		return "unknown"
	}
	return version
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
	err   error
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.err = errOutputLimit
		return 0, b.err
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.err = errOutputLimit
		return remaining, b.err
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte  { return b.buf.Bytes() }
func (b *limitedBuffer) String() string { return b.buf.String() }
