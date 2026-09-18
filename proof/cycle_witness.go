// Cycle witnesses split the entry prefix from the repeatable nonterminal cycle.
// They never report a finite prefix as an infinite execution.
package proof

func cycleWitness(index graphIndex, requirementID string, counterexample cycleCounterexample) *Witness {
	cycle := traceFor(index, counterexample.cycle)
	return &Witness{
		Kind: WitnessTerminationCycle, RootID: counterexample.prefix.rootID,
		RequirementID: requirementID,
		Cause:         "reachable execution repeats without entering a terminal state",
		StateID:       lastStateID(counterexample.prefix),
		Path:          traceFor(index, counterexample.prefix), Cycle: &cycle,
	}
}
