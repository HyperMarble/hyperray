// Solver-status parsing keeps process text separate from semantic results.
// It never accepts extra status text or a solver-incomplete answer.
package circuit

func solverStatus(output string) (string, error) {
	tokens := solverTokens(output)
	if len(tokens) == 0 {
		return "", engineError("empty_solver_output", "status")
	}
	if len(tokens) != 1 {
		return "", engineError("unexpected_solver_output", "status")
	}
	return tokens[0], nil
}

func proposalFromTokens(
	tokens []string,
	miter Miter,
	identity ToolIdentity,
) (Proposal, error) {
	base := Proposal{Tool: identity, MiterDigest: miter.digest}
	switch tokens[0] {
	case "unsat":
		base.Status = UnvalidatedUnsat
		return base, nil
	case "sat":
		return differenceProposal(base, miter, tokens[1:])
	case "unknown":
		return Proposal{}, engineError("solver_incomplete", identity.Path)
	default:
		return Proposal{}, engineError("unsupported_solver_output", tokens[0])
	}
}
