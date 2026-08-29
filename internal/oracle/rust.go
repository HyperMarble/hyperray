package oracle

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// verusResultRe matches Verus's real summary line, e.g.
// "verification results:: 2 verified, 0 errors" -- capturing the error
// count is what matters: a real stress test found the previous exact-string
// match on "1 verified, 0 errors" misclassified any file with more than
// one verified item (a multi-function model, or a model plus a separate
// `proof fn`) as REFUTED, because the generic "errors" fallback below also
// matches inside "0 errors".
var verusResultRe = regexp.MustCompile(`verification results:: \d+ verified, (\d+) errors`)

// ProveRust runs a complete Verus source file (the caller writes real
// `requires`/`ensures` clauses using Verus's own syntax, inside a `verus!
// { ... }` block, the same way this was verified hands-on tonight) through
// the given verus binary.
func ProveRust(verusPath, src string) (Verdict, error) {
	if verusPath == "" {
		verusPath = "verus"
	}
	if _, err := exec.LookPath(verusPath); err != nil {
		return Verdict{}, fmt.Errorf("verus binary not found (%q): %w", verusPath, err)
	}

	f, err := os.CreateTemp("", "hyperray-verus-model-*.rs")
	if err != nil {
		return Verdict{}, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(src); err != nil {
		f.Close()
		return Verdict{}, err
	}
	if err := f.Close(); err != nil {
		return Verdict{}, err
	}

	cmd := exec.Command(verusPath, f.Name())
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // verus exits non-zero on a failed postcondition; not a Go error

	text := out.String()
	if m := verusResultRe.FindStringSubmatch(text); m != nil {
		if m[1] == "0" {
			return Verdict{Status: "PROVED"}, nil
		}
		return Verdict{Status: "REFUTED", Reason: text}, nil
	}
	return Verdict{Status: "UNKNOWN", Reason: text}, nil
}
