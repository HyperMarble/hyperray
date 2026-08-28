// Package difftest implements ray's Layer 4: it runs the real solution
// and the proven oracle model on the same concrete inputs and reports
// where they disagree.
//
// Layer 3 proves a property of a simplified reference model. Nothing in
// that proof establishes that the shipped implementation matches the
// model -- a typo or an off-by-one in the real code diverges silently
// while the proof stays valid. This layer is what catches that drift,
// and it is the step that turns "we proved something" into evidence
// about the code that actually ships.
package difftest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Observation is what one side did on one input: it either returned a
// value or raised an exception.
type Observation struct {
	Outcome       string `json:"outcome"` // "return" or "raise"
	Value         any    `json:"value,omitempty"`
	ExceptionType string `json:"exception_type,omitempty"`
	Message       string `json:"message,omitempty"`
}

// Disagreement is one concrete input on which the model and the real
// solution behaved differently -- the actual finding this layer exists
// to produce.
type Disagreement struct {
	Input []any       `json:"input"`
	Model Observation `json:"model"`
	Real  Observation `json:"real"`
}

// Result is the outcome of one diff-test run.
type Result struct {
	Total      int `json:"total"`
	Agreements int `json:"agreements"`
	// ReturnedNormally counts inputs on which the model returned a value
	// rather than raising. Agreement across inputs that only ever raised
	// is not evidence of equivalent behaviour -- it can mean the inputs
	// never fit the function's signature, so nothing was exercised.
	ReturnedNormally int            `json:"returned_normally"`
	Disagreements    []Disagreement `json:"disagreements"`
}

// Pass reports whether the real solution agreed with the proven model on
// every input tried. A pass is not proof of correctness -- it is finite
// evidence over the inputs actually run, which is exactly why the inputs
// matter and why dep-harvest feeds real edge cases in.
func (r Result) Pass() bool { return len(r.Disagreements) == 0 }

type request struct {
	ModelSrc string  `json:"model_src"`
	ModelFn  string  `json:"model_fn"`
	RealSrc  string  `json:"real_src"`
	RealFn   string  `json:"real_fn"`
	Inputs   [][]any `json:"inputs"`
}

// driverPath resolves difftest_driver.py relative to this source file so
// Run works regardless of the caller's working directory.
func driverPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("difftest: could not resolve driver script path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..",
		"third_party", "difftest", "difftest_driver.py"), nil
}

// Run executes modelSrc's modelFn and realSrc's realFn on every input and
// reports disagreements. Each input is one argument list.
//
// pythonPath need not be the patched oracle venv -- this layer executes
// real code rather than proving anything, so any interpreter that can
// import the solution's dependencies is appropriate.
func Run(pythonPath, modelSrc, modelFn, realSrc, realFn string, inputs [][]any) (Result, error) {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if _, err := exec.LookPath(pythonPath); err != nil {
		return Result{}, fmt.Errorf("python interpreter not found (%q): %w", pythonPath, err)
	}
	driver, err := driverPath()
	if err != nil {
		return Result{}, err
	}
	if inputs == nil {
		inputs = [][]any{}
	}

	reqJSON, err := json.Marshal(request{
		ModelSrc: modelSrc, ModelFn: modelFn,
		RealSrc: realSrc, RealFn: realFn, Inputs: inputs,
	})
	if err != nil {
		return Result{}, err
	}

	cmd := exec.Command(pythonPath, driver)
	cmd.Stdin = bytes.NewReader(reqJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("difftest driver: %w: %s", err, stderr.String())
	}

	var res Result
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return Result{}, fmt.Errorf("parsing difftest driver output: %w", err)
	}
	return res, nil
}
