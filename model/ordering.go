// Validation ordering makes the first model error independent of input order.
// It never changes the caller's slices or state-value maps.
package model

import "sort"

func orderedForValidation(graph Model) Model {
	ordered := Model{
		States:      append([]State(nil), graph.States...),
		Transitions: append([]Transition(nil), graph.Transitions...),
	}
	sort.Slice(ordered.States, func(left int, right int) bool {
		return stateBefore(ordered.States[left], ordered.States[right])
	})
	sort.Slice(ordered.Transitions, func(left int, right int) bool {
		return transitionBefore(ordered.Transitions[left], ordered.Transitions[right])
	})
	return ordered
}

func stateBefore(left State, right State) bool {
	if left.ID != right.ID {
		return left.ID < right.ID
	}
	_, leftEmpty := left.Values[""]
	_, rightEmpty := right.Values[""]
	return leftEmpty && !rightEmpty
}

func transitionBefore(left Transition, right Transition) bool {
	if left.ID != right.ID {
		return left.ID < right.ID
	}
	if left.FromStateID != right.FromStateID {
		return left.FromStateID < right.FromStateID
	}
	return left.ToStateID < right.ToStateID
}
