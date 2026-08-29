package oracle

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// cexRe matches ESBMC's counterexample state lines, e.g.
// "  x = 1 (00000000 ...)" -- used to build a compact counterexample string
// since ESBMC's plain-text output has no single "here's the model" line the
// way touchstone's Verdict.counterexample does.
var cexRe = regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*) = (-?\d+)`)

// ProveCPP runs a complete C/C++ source file (the caller embeds the property
// as assert() calls and any preconditions as __ESBMC_assume(), the same way
// this was verified hands-on tonight -- there's no separate ensures/requires
// string API the way touchstone has, because assert/assume already are that
// API in C) through ESBMC, unwound to the given bound.
func ProveCPP(esbmcPath, src string, unwind int) (Verdict, error) {
	if esbmcPath == "" {
		esbmcPath = "esbmc"
	}
	if _, err := exec.LookPath(esbmcPath); err != nil {
		return Verdict{}, fmt.Errorf("esbmc binary not found (%q): %w", esbmcPath, err)
	}
	if unwind <= 0 {
		unwind = 10
	}

	f, err := os.CreateTemp("", "hyperray-esbmc-model-*.c")
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

	cmd := exec.Command(esbmcPath, f.Name(), "--unwind", fmt.Sprintf("%d", unwind))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	_ = cmd.Run() // ESBMC exits non-zero on VERIFICATION FAILED; that's not a Go error

	out := stdout.String()
	switch {
	case bytes.Contains(stdout.Bytes(), []byte("VERIFICATION SUCCESSFUL")):
		return Verdict{Status: "PROVED"}, nil
	case bytes.Contains(stdout.Bytes(), []byte("VERIFICATION FAILED")):
		v := Verdict{Status: "REFUTED"}
		if m := cexRe.FindAllStringSubmatch(out, -1); len(m) > 0 {
			parts := make([]string, len(m))
			for i, g := range m {
				parts[i] = g[1] + "=" + g[2]
			}
			seen := map[string]bool{}
			var uniq []string
			for _, p := range parts {
				if !seen[p] {
					seen[p] = true
					uniq = append(uniq, p)
				}
			}
			v.Counterexample = fmt.Sprintf("%v", uniq)
		}
		return v, nil
	default:
		return Verdict{Status: "UNKNOWN", Reason: out}, nil
	}
}
