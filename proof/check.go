// Check proves one requirement over the graph inside a coverage capability.
// It never accepts caller-selected entry states or a partial coverage report.
package proof

import "github.com/HyperMarble/hyperray/coverage"

func Check(validated coverage.ValidatedCoverage, query Query) (Result, error) {
	report, err := validated.Report()
	if err != nil {
		return Result{}, coverageFailure(err)
	}
	if !report.Complete {
		return Result{}, proofError("incomplete_coverage", "report.complete")
	}
	graph, err := validated.Model()
	if err != nil {
		return Result{}, coverageFailure(err)
	}
	roots, err := validated.Roots(query.RootIDs)
	if err != nil {
		return Result{}, coverageFailure(err)
	}
	index := indexGraph(graph)
	sets, err := validateRequirement(index, query.Requirement)
	if err != nil {
		return Result{}, err
	}
	reachable := explore(index, roots)
	switch query.Requirement.Kind {
	case RequirementSafety:
		return proveSafety(index, reachable, report, query.Requirement, sets), nil
	case RequirementTotalTermination:
		return proveTermination(index, reachable, report, query.Requirement, sets), nil
	default:
		return Result{}, proofError("unknown_requirement_kind", string(query.Requirement.Kind))
	}
}

func baseResult(verdict Verdict, report coverage.Report, requirement Requirement, reachable reachability) Result {
	return Result{
		Verdict: verdict, RequirementID: requirement.ID, Coverage: report,
		ReachableStateIDs:      append([]string(nil), reachable.stateIDs...),
		ReachableTransitionIDs: append([]string(nil), reachable.transitionIDs...),
	}
}
