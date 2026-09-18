// Canonical ordering gives model validation a stable view of caller input.
// It never mutates the caller's graph.
package coverage

import (
	"sort"

	"github.com/HyperMarble/hyperray/model"
)

func canonicalModel(graph model.Model) model.Model {
	ordered := cloneModel(graph)
	sort.Slice(ordered.States, func(left int, right int) bool {
		if ordered.States[left].ID != ordered.States[right].ID {
			return ordered.States[left].ID < ordered.States[right].ID
		}
		_, leftEmpty := ordered.States[left].Values[""]
		_, rightEmpty := ordered.States[right].Values[""]
		return leftEmpty && !rightEmpty
	})
	sort.Slice(ordered.Transitions, func(left int, right int) bool {
		first := ordered.Transitions[left]
		second := ordered.Transitions[right]
		if first.ID != second.ID {
			return first.ID < second.ID
		}
		if first.FromStateID != second.FromStateID {
			return first.FromStateID < second.FromStateID
		}
		return first.ToStateID < second.ToStateID
	})
	return ordered
}

func cloneModel(graph model.Model) model.Model {
	result := model.Model{
		States:      append([]model.State(nil), graph.States...),
		Transitions: append([]model.Transition(nil), graph.Transitions...),
	}
	for index := range result.States {
		if graph.States[index].Values == nil {
			continue
		}
		result.States[index].Values = make(map[string]string, len(graph.States[index].Values))
		for key, value := range graph.States[index].Values {
			result.States[index].Values[key] = value
		}
	}
	return result
}
