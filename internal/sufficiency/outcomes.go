package sufficiency

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Outcome is one distinct terminal point in a real source function --
// either a `return` or a `raise`/`throw`/`panic`, extracted uniformly by
// extract_outcomes.py from that language's own tree-sitter grammar.
//
// SourceText is the matched statement's verbatim source, whatever its
// shape. There is deliberately no derived ExceptionType or Message
// field: an earlier version parsed those out with hand-written node-type
// lists and string munging, and matching spec.md's quoted phrases
// against the verbatim text was confirmed against the real sktime source
// to give the identical result -- so all of that was redundant.
type Outcome struct {
	Kind       string `json:"kind"` // "raise" or "return"
	Line       int    `json:"line"`
	SourceText string `json:"source_text"`
}

// Language names extract_outcomes.py accepts -- ray's four target
// languages, each backed by that language's own real tree-sitter
// grammar rather than by parsing rules of ray's own.
const (
	LangPython = "python"
	LangRust   = "rust"
	LangCPP    = "cpp"
	LangGo     = "go"
)

// ExtractOutcomes shells out to
// third_party/branch-extract/extract_outcomes.py against the real
// source file and returns every return/raise it found. language must be
// one of the Lang* constants; the script uses that language's real
// tree-sitter grammar, so this works identically across all of ray's
// targets rather than being Python-only.
//
// pythonPath is only the interpreter that runs the extractor script
// itself (which needs the tree_sitter bindings installed) -- it is
// unrelated to the language of the source being analyzed.
//
// A bare re-raise and a bare `return` are excluded by the queries
// themselves: neither carries text spec.md could ever match, so
// reporting them would be a permanent, unfixable gap.
func ExtractOutcomes(pythonPath, extractScriptPath, sourcePath, language string) ([]Outcome, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if _, err := exec.LookPath(pythonPath); err != nil {
		return nil, fmt.Errorf("python interpreter not found (%q): %w", pythonPath, err)
	}
	switch language {
	case LangPython, LangRust, LangCPP, LangGo:
	default:
		return nil, fmt.Errorf("unsupported language %q", language)
	}
	cmd := exec.Command(pythonPath, extractScriptPath, sourcePath, language)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("extract_outcomes.py: %w: %s", err, stderr.String())
	}

	var outcomes []Outcome
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var o Outcome
		if err := dec.Decode(&o); err != nil {
			return nil, fmt.Errorf("parsing extract_outcomes.py output: %w", err)
		}
		outcomes = append(outcomes, o)
	}
	return outcomes, nil
}

// ScriptPath resolves extract_outcomes.py relative to this source file,
// so callers work regardless of the current working directory. An
// earlier version resolved it relative to the process's cwd, which meant
// `ray start` silently skipped the sufficiency pass whenever it was run
// from anywhere but the repo root.
func ScriptPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("sufficiency: could not resolve extractor path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..",
		"third_party", "branch-extract", "extract_outcomes.py"), nil
}
