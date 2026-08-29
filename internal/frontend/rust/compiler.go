package rust

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// validateWithRustc makes rustc, rather than the source parser, authoritative
// for name resolution, type checking, macro expansion, and control-flow
// lowering. The parser is retained for source spans and the closed IR mapping.
type rustCompilerOutput struct {
	MIR               string
	MIRStderr         []byte
	LLVM              string
	Argv              []string
	WorkingDirectory  string
	EnvironmentDigest string
}

func validateWithRustc(ctx context.Context, request semanticir.FrontendRequest, functions []functionDecl) (rustCompilerOutput, []semanticir.Diagnostic) {
	whole := wholeSpan(request.Source)
	tool := request.Translator
	var diagnostics []semanticir.Diagnostic
	if tool.Name != "rustc" {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, fmt.Sprintf("Rust translation requires translator name rustc, got %q", tool.Name)))
	}
	if tool.Path == "" || !filepath.IsAbs(tool.Path) {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "rustc translator path must be absolute"))
	}
	if !semanticir.ValidDigest(tool.Digest) {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "rustc translator digest is missing or malformed"))
	}
	if tool.Version == "" {
		diagnostics = append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "rustc translator version is missing"))
	}
	if semanticir.HasErrors(diagnostics) {
		return rustCompilerOutput{}, diagnostics
	}
	binary, err := os.ReadFile(tool.Path)
	if err != nil {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, "read pinned rustc: "+err.Error()))
	}
	if actual := semanticir.DigestBytes(binary); actual != tool.Digest {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticStaleArtifact, fmt.Sprintf("rustc digest mismatch: pinned %s, actual %s", tool.Digest, actual)))
	}
	versionCommand := exec.CommandContext(ctx, tool.Path, "--version", "--verbose")
	configureRustCommand(versionCommand, request.Workspace)
	versionBytes, err := versionCommand.CombinedOutput()
	if err != nil {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidReference, fmt.Sprintf("run pinned rustc --version --verbose: %v: %s", err, strings.TrimSpace(string(versionBytes)))))
	}
	actualVersion := strings.TrimSpace(string(versionBytes))
	if actualVersion != tool.Version {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticStaleArtifact, "rustc reported version does not match the frozen translator version"))
	}

	tempDir, err := os.MkdirTemp("", "hyperray-rust-frontend-*")
	if err != nil {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "create rustc workspace: "+err.Error()))
	}
	defer os.RemoveAll(tempDir)
	sourcePath := filepath.Join(tempDir, "artifact.rs")
	workspaceSource, inWorkspace := rustWorkspaceSource(request)
	if inWorkspace {
		sourcePath = workspaceSource
	} else if err := os.WriteFile(sourcePath, request.Source, 0o600); err != nil {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticInvalidInput, "write rustc input: "+err.Error()))
	}
	baseArgs := []string{"--edition=2021", "--crate-name", "ray_frontend_artifact", "-C", "link-dead-code=yes"}
	if request.Kind == semanticir.ArtifactTests {
		baseArgs = append(baseArgs, "--test")
	} else {
		baseArgs = append(baseArgs, "--crate-type=lib")
	}
	// These arguments are intentionally fixed.  MIR is used only to bind
	// source constructs to resolved compiler bodies; LLVM IR is the compiler
	// semantic evidence consumed before any bounded executions are accepted.
	workDirectory := ""
	sourceArgument := sourcePath
	if directory, ok := withinRustWorkspace(request.Workspace.Root, request.Workspace.WorkingDirectory); ok && inWorkspace {
		workDirectory = directory
		if relativeSource, relativeErr := filepath.Rel(directory, sourcePath); relativeErr == nil {
			sourceArgument = filepath.ToSlash(relativeSource)
		}
	}
	runEmit := func(kind string) ([]byte, []byte, []string, error) {
		args := append(append([]string(nil), baseArgs...), "--emit="+kind+"=-", sourceArgument)
		command := exec.CommandContext(ctx, tool.Path, args...)
		configureRustCommand(command, request.Workspace)
		command.Dir = workDirectory
		var stdout, stderr strings.Builder
		command.Stdout, command.Stderr = &stdout, &stderr
		runErr := command.Run()
		if runErr != nil && stderr.Len() != 0 {
			return nil, nil, args, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return []byte(stdout.String()), []byte(stderr.String()), args, runErr
	}
	mirBytes, mirStderr, mirArgs, err := runEmit("mir")
	if err != nil {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticUnsupported, "rustc could not emit MIR for the frozen Rust artifact: "+err.Error()))
	}
	mir := string(mirBytes)
	diagnostics = append(diagnostics, auditMIR(request.Artifact, functions, mir)...)
	llvmBytes, _, _, err := runEmit("llvm-ir")
	if err != nil {
		return rustCompilerOutput{}, append(diagnostics, diagnostic(request.Artifact, whole, semanticir.DiagnosticUnsupported, "rustc could not emit LLVM IR for the frozen Rust artifact: "+err.Error()))
	}
	diagnostics = append(diagnostics, auditLLVM(request.Artifact, functions, string(llvmBytes))...)
	return rustCompilerOutput{MIR: mir, MIRStderr: mirStderr, LLVM: string(llvmBytes), Argv: append([]string{tool.Path}, mirArgs...), WorkingDirectory: workDirectory, EnvironmentDigest: request.Workspace.EnvironmentDigest}, diagnostics
}

func auditLLVM(artifact semanticir.ArtifactRef, functions []functionDecl, llvm string) []semanticir.Diagnostic {
	whole := sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Line: 1, Column: 1}}
	if !strings.Contains(llvm, "source_filename =") || !strings.Contains(llvm, "define ") {
		return []semanticir.Diagnostic{diagnostic(artifact, whole, semanticir.DiagnosticUnsupported, "pinned rustc LLVM IR is empty or lacks function definitions")}
	}
	var diagnostics []semanticir.Diagnostic
	for _, fn := range functions {
		// Rust symbol mangling retains the source identifier.  Requiring it in
		// the emitted module prevents a syntax-only function from being treated
		// as compiler-backed semantic evidence.
		if !strings.Contains(llvm, fn.Name) {
			diagnostics = append(diagnostics, diagnostic(artifact, fn.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("pinned rustc LLVM IR has no emitted symbol for %s", fn.Name)))
		}
	}
	return diagnostics
}

func auditMIR(artifact semanticir.ArtifactRef, functions []functionDecl, mir string) []semanticir.Diagnostic {
	var diagnostics []semanticir.Diagnostic
	for _, fn := range functions {
		section, ok := mirFunction(mir, fn.Name)
		if !ok {
			diagnostics = append(diagnostics, diagnostic(artifact, fn.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("rustc MIR has no uniquely resolved body for function %s", fn.Name)))
			continue
		}
		facts := collectSyntaxFacts(fn.Body)
		checks := []struct {
			needed bool
			marker string
			label  string
		}{
			{facts.branch, "switchInt(", "if/match branch"},
			{facts.comparison, "Eq(", "comparison"},
			{facts.result, "Result::<", "Result constructor"},
			{facts.panic, "panic_fmt", "panic outcome"},
			{facts.boundedLoop, "Iterator>::next", "bounded for-loop"},
		}
		for _, check := range checks {
			if check.needed && !strings.Contains(section, check.marker) {
				// Comparison operations have distinct MIR operator names.
				if check.label == "comparison" && containsAny(section, "Ne(", "Lt(", "Le(", "Gt(", "Ge(", "switchInt(") {
					continue
				}
				// Result construction can be printed without the fully-qualified
				// type when inference has already fixed it. In test assertions the
				// constructor literal is emitted in a function-owned promoted const,
				// while the test body contains the resolved Result equality call.
				if check.label == "Result constructor" && (containsAny(section, "::Ok(", "::Err(") || (strings.Contains(section, "Result<") && mirHasOwnedResultPromotion(mir, fn.Name))) {
					continue
				}
				diagnostics = append(diagnostics, diagnostic(artifact, fn.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("cannot map %s in %s to pinned rustc MIR", check.label, fn.Name)))
			}
		}
		for _, call := range facts.calls {
			if !strings.Contains(section, call+"(") && call != "Ok" && call != "Err" && call != "Result::Ok" && call != "Result::Err" {
				diagnostics = append(diagnostics, diagnostic(artifact, fn.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("cannot resolve call %s in pinned rustc MIR for %s", call, fn.Name)))
			}
		}
		if facts.assertion && !containsAny(section, "assert_failed", "panic(", "begin_panic") {
			diagnostics = append(diagnostics, diagnostic(artifact, fn.Span, semanticir.DiagnosticUnsupported, fmt.Sprintf("cannot map test assertion in %s to pinned rustc MIR", fn.Name)))
		}
	}
	return diagnostics
}

func mirHasOwnedResultPromotion(mir, functionName string) bool {
	marker := "const " + functionName + "::promoted["
	for offset := 0; ; {
		start := strings.Index(mir[offset:], marker)
		if start < 0 {
			return false
		}
		start += offset
		remainder := mir[start:]
		end := strings.Index(remainder, "\n}\n")
		if end < 0 {
			end = len(remainder)
		}
		section := remainder[:end]
		if strings.Contains(section, "Result::<") && containsAny(section, "::Ok(", "::Err(") {
			return true
		}
		offset = start + len(marker)
	}
}

func mirFunction(mir, name string) (string, bool) {
	marker := "fn " + name + "("
	start := strings.Index(mir, marker)
	if start < 0 {
		return "", false
	}
	if strings.Index(mir[start+len(marker):], marker) >= 0 {
		return "", false
	}
	// rustc may print const/static items between function definitions. A MIR
	// function itself always ends with a top-level closing brace, while basic
	// block braces are indented. Stop there so unrelated following items cannot
	// change the compiler identity of the function body.
	remainder := mir[start:]
	end := strings.Index(remainder, "\n}\n")
	if end < 0 {
		if strings.HasSuffix(remainder, "\n}") {
			return remainder, true
		}
		return "", false
	}
	return remainder[:end+2], true
}

type syntaxFacts struct {
	branch      bool
	comparison  bool
	result      bool
	panic       bool
	assertion   bool
	boundedLoop bool
	calls       []string
}

func collectSyntaxFacts(value block) syntaxFacts {
	var facts syntaxFacts
	var visitExpr func(expression)
	visitBlock := func(value block) {}
	var visitBlockImpl func(block)
	visitExpr = func(expr expression) {
		if expr.Kind == expressionIf || expr.Kind == expressionMatch {
			facts.branch = true
		}
		if expr.Kind == expressionBinary && isComparison(expr.Text) {
			facts.comparison = true
		}
		if expr.Kind == expressionCall {
			facts.calls = appendUnique(facts.calls, expr.Text)
			if expr.Text == "Ok" || expr.Text == "Err" || expr.Text == "Result::Ok" || expr.Text == "Result::Err" {
				facts.result = true
			}
		}
		if expr.Kind == expressionMacro {
			facts.panic = facts.panic || expr.Text == "panic"
			facts.assertion = facts.assertion || isAssertionMacro(expr.Text)
		}
		for _, child := range expr.Children {
			visitExpr(child)
		}
		if expr.Then != nil {
			visitBlockImpl(*expr.Then)
		}
		if expr.Else != nil {
			visitBlockImpl(*expr.Else)
		}
		for _, arm := range expr.Arms {
			if arm.Guard != nil {
				visitExpr(*arm.Guard)
			}
			visitExpr(arm.Value)
		}
	}
	visitBlockImpl = func(value block) {
		for _, stmt := range value.Statements {
			if stmt.Kind == statementFor {
				facts.boundedLoop = true
			}
			visitExpr(stmt.Expr)
			if stmt.Body != nil {
				visitBlockImpl(*stmt.Body)
			}
		}
		if value.Tail != nil {
			visitExpr(*value.Tail)
		}
	}
	visitBlock = visitBlockImpl
	visitBlock(value)
	return facts
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
