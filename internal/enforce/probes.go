package enforce

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Probes come from a declared input SHAPE, not from a hand-written list.
//
// Which inputs get chosen decides everything: on the cron fixture, four
// hand-picked probes found zero false positives and ten found nine. Left
// to a person, that choice is a guess, and the whole result rests on it.
//
// So the task declares the shape of an input once and Hypothesis produces
// the concrete ones, biased toward the values that expose behaviour --
// empty, zero, boundaries, reversed.
//
// A solver was tried in place of this and removed. CrossHair's
// `diffbehavior` decides the same question symbolically and returns an
// exact witness, which is strictly better on a plain typed function --
// measured, it found the boundary n == 100 in 0.7 seconds. On this corpus
// it is not usable: every real task takes strings or structured objects,
// and a symbolic string through split/partition/int stalls. It returned
// "unknown" after 183 iterations on the very adversary probes catch at
// once. Kept out rather than kept as a fallback, because a 15-second
// timeout per adversary buys nothing here.

// Probe is an input the adversary and the real solution are both run on.
type Probe struct {
	Name    string
	Command string
	// Path is the probe's script file, set when the probes come from the
	// spec's bridges directory. It enables batch observation: one process
	// runs every script, paying the interpreter/import cost once, with each
	// probe still executed in full.
	Path string
}

// ProbeShapeScript returns the path to the generator, resolved relative to
// this file so it works from any working directory.
func ProbeShapeScript() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("enforce: could not resolve the probe generator's path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..",
		"third_party", "probes", "generate_probes.py"), nil
}

// GenerateProbes turns a declared input shape into concrete probes.
//
// Each generated input is substituted into commandTemplate at {input}, so
// the task keeps ownership of how an input reaches the program -- a
// command line argument, a file, stdin. Nothing here knows the task's
// domain.
//
// mustInclude carries values the caller wants guaranteed. Boundary values
// the code itself names belong here: a constant an adversary shifts from
// 59 to 60 is the source stating that 59 matters, which no generator can
// know from the input shape alone.
func GenerateProbes(pythonPath, shape, commandTemplate string, count int, mustInclude []string) ([]Probe, error) {
	if shape == "" || commandTemplate == "" {
		return nil, nil
	}
	if pythonPath == "" {
		pythonPath = "python3"
	}
	script, err := ProbeShapeScript()
	if err != nil {
		return nil, err
	}
	args := []string{script, shape, "--count", fmt.Sprint(count)}
	for _, m := range mustInclude {
		args = append(args, "--must-include", m)
	}

	out, err := exec.Command(pythonPath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("probe generation failed (is hypothesis installed?): %w", err)
	}

	var probes []Probe
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var input string
		if err := json.Unmarshal([]byte(line), &input); err != nil {
			continue
		}
		probes = append(probes, Probe{
			Name:    shorten(oneLine(input)),
			Command: strings.ReplaceAll(commandTemplate, "{input}", shellQuote(input)),
		})
	}
	return probes, nil
}

// shellQuote wraps a generated input for safe use in a command. Generated
// inputs contain quotes, newlines and arbitrary unicode by design -- that
// is the point of them -- so this must never be skipped.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
