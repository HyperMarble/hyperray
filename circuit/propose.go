// Proposal execution sends one immutable miter to one identified engine.
// It never maps process or parsing errors to an equivalence result.
package circuit

import (
	"context"
	"os/exec"
	"strings"
)

// Propose requests a difference or raw UNSAT without certifying either.
func (tool DifferenceEngine) Propose(ctx context.Context, miter Miter) (Proposal, error) {
	if ctx == nil {
		return Proposal{}, engineError("nil_context", "proposal")
	}
	if tool.identity.Path == "" || tool.identity.Version == "" || tool.identity.Digest == "" {
		return Proposal{}, engineError("unidentified_tool", "z3")
	}
	if err := miterError(miter); err != nil {
		return Proposal{}, err
	}
	output, err := runEngine(ctx, tool.identity.Path, miter.smt2)
	if err != nil {
		return Proposal{}, err
	}
	status, err := solverStatus(output)
	if err != nil {
		return Proposal{}, err
	}
	if status != "sat" {
		return proposalFromTokens(solverTokens(output), miter, tool.identity)
	}
	witnessOutput, err := runEngine(ctx, tool.identity.Path, witnessInput(miter))
	if err != nil {
		return Proposal{}, err
	}
	witnessTokens := solverTokens(witnessOutput)
	if len(witnessTokens) == 0 || witnessTokens[0] != "sat" {
		return Proposal{}, engineError("solver_result_changed", status)
	}
	return proposalFromTokens(witnessTokens, miter, tool.identity)
}

func runEngine(ctx context.Context, path string, input string) (string, error) {
	command := exec.CommandContext(ctx, path, "-in")
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", engineError("solver_process_error", err.Error(), strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
