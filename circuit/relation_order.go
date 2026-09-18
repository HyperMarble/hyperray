// Canonical relation order makes miter bytes independent of input order.
// Ordering never changes expression semantics or caller-owned slices.
package circuit

import "sort"

func orderRelation(relation *StepRelation) {
	sort.Slice(relation.states, func(left int, right int) bool {
		return relation.states[left].name < relation.states[right].name
	})
	sort.Slice(relation.inputs, func(left int, right int) bool {
		return relation.inputs[left].name < relation.inputs[right].name
	})
	sort.Slice(relation.nextStates, func(left int, right int) bool {
		return relation.nextStates[left].name < relation.nextStates[right].name
	})
	sort.Slice(relation.observations, func(left int, right int) bool {
		return relation.observations[left].name < relation.observations[right].name
	})
}
