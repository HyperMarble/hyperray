// Tool-error tests exercise public discovery and proposal failure paths.
// They never convert a tool fault into a semantic result.
package circuit_test

import (
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestToolErrors(t *testing.T) {
	_, err := circuit.NewDifferenceEngine("/hyperray/missing/proposal-engine")
	problem := requireEngineCode(t, err, "tool_not_found")
	if len(problem.References) != 2 || problem.References[0] != "/hyperray/missing/proposal-engine" {
		t.Errorf("tool-not-found references = %q", problem.References)
	}
	_, err = circuit.NewDifferenceEngine("/usr/bin/false")
	requireEngineError(t, err, "tool_identity_error", "/usr/bin/false", "exit status 1")
	_, err = circuit.NewDifferenceEngine("/bin/echo")
	requireEngineError(t, err, "unsupported_tool_identity", "/bin/echo", "--version")
	_, err = circuit.NewDifferenceEngine("../fixtures/circuit/z3-proposal-fixture.sh")
	requireEngineError(t, err, "tool_path_not_absolute", "../fixtures/circuit/z3-proposal-fixture.sh")
	requireEngineError(t, proposalErrorForMode(t, "process_error"),
		"solver_process_error", "exit status 7", "fixture process error")
	requireEngineError(t, proposalErrorForMode(t, "witness_error"),
		"solver_process_error", "exit status 8", "fixture witness error")
	requireEngineError(t, proposalErrorForMode(t, "changed"),
		"solver_result_changed", "sat")
}

func TestUnsupportedSolverOutput(t *testing.T) {
	requireEngineError(t, proposalErrorForMode(t, "unsupported"),
		"unsupported_solver_output", "maybe")
	path := fixtureToolPath(t)
	requireEngineError(t, proposalErrorForMode(t, "unknown"), "solver_incomplete", path)
	requireEngineError(t, proposalErrorForMode(t, "empty"), "empty_solver_output", "status")
	requireEngineError(t, proposalErrorForMode(t, "incomplete"),
		"incomplete_solver_model", "get-value")
	requireEngineError(t, proposalErrorForMode(t, "duplicate"),
		"duplicate_solver_value", "|state.balance|")
	requireEngineError(t, proposalErrorForMode(t, "extra"),
		"unexpected_solver_value", "|extra|")
	requireEngineError(t, proposalErrorForMode(t, "invalid"),
		"invalid_solver_value", "|state.balance|", "zero")
	requireEngineError(t, proposalErrorForMode(t, "equal"),
		"solver_sat_without_difference", "miter")
	requireEngineError(t, proposalErrorForMode(t, "status_extra"),
		"unexpected_solver_output", "status")
	for _, mode := range []string{"malformed_outer", "malformed_short", "malformed_pair"} {
		requireEngineError(t, proposalErrorForMode(t, mode),
			"malformed_solver_model", "get-value")
	}
}

func fixtureToolPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../fixtures/circuit/z3-proposal-fixture.sh")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	return path
}
