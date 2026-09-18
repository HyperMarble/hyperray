// Difference construction maps private solver symbols to public step names.
// It never reports SAT when all requested outputs have equal values.
package circuit

func differenceProposal(
	base Proposal,
	miter Miter,
	tokens []string,
) (Proposal, error) {
	values, err := solverValues(tokens, expectedSymbols(miter))
	if err != nil {
		return Proposal{}, err
	}
	if err := solverValueTypesError(miter, values); err != nil {
		return Proposal{}, err
	}
	difference, err := firstDifference(miter, values)
	if err != nil {
		return Proposal{}, err
	}
	base.Status = DifferenceFound
	base.Difference = &difference
	return base, nil
}

func firstDifference(miter Miter, values map[string]string) (Difference, error) {
	assignments := make([]Assignment, 0, len(miter.assignments))
	for _, query := range miter.assignments {
		assignments = append(assignments, Assignment{
			Role: query.role, Name: query.name, Value: values[query.symbol],
		})
	}
	for _, output := range miter.outputs {
		referenceValue := values[output.referenceSymbol]
		candidateValue := values[output.candidateSymbol]
		if referenceValue == candidateValue {
			continue
		}
		return Difference{
			Assignments: assignments, Kind: output.kind, Name: output.name,
			ReferenceValue: referenceValue, CandidateValue: candidateValue,
		}, nil
	}
	return Difference{}, engineError("solver_sat_without_difference", "miter")
}
