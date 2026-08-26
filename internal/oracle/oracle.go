// Package oracle proves a model's properties using touchstone-prover
// (patched, see third_party/touchstone-patch), an SMT-based prover that
// mathematically proves a property holds for every possible input in one
// symbolic computation, rather than testing finite concrete inputs.
package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Verdict is touchstone's real verdict on one property, unmarshaled from
// oracle_driver.py's JSON response.
type Verdict struct {
	Status         string `json:"status"` // "PROVED", "REFUTED", or "UNKNOWN"
	Reason         string `json:"reason"`
	Counterexample string `json:"counterexample"`
}

type request struct {
	Src        string `json:"src"`
	Ensures    string `json:"ensures"`
	Requires   string `json:"requires,omitempty"`
	BestEffort *bool  `json:"best_effort,omitempty"`
}

// driverPath is oracle_driver.py's location relative to this source file,
// so Prove finds it regardless of the caller's working directory.
func driverPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("oracle: could not resolve driver script path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "third_party", "touchstone-patch", "oracle_driver.py"), nil
}

// Prove runs src (a single Python function definition) through the patched
// touchstone-prover, checking that ensures holds for every input meeting
// requires (default "True"). pythonPath is the interpreter from a venv
// built by third_party/touchstone-patch/build.sh -- the patched
// touchstone-prover isn't bundled or on a bare python3's default path,
// same honest v0.1.0 simplification as coverage assuming a pict binary.
func Prove(pythonPath, src, ensures, requires string) (Verdict, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if _, err := exec.LookPath(pythonPath); err != nil {
		return Verdict{}, fmt.Errorf("python interpreter not found (%q): %w", pythonPath, err)
	}
	driver, err := driverPath()
	if err != nil {
		return Verdict{}, err
	}

	req := request{Src: src, Ensures: ensures, Requires: requires}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return Verdict{}, err
	}

	cmd := exec.Command(pythonPath, driver)
	cmd.Stdin = bytes.NewReader(reqJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Verdict{}, fmt.Errorf("oracle driver: %w: %s", err, stderr.String())
	}

	var v Verdict
	if err := json.Unmarshal(stdout.Bytes(), &v); err != nil {
		return Verdict{}, fmt.Errorf("parsing oracle driver output: %w", err)
	}
	return v, nil
}
