package testir

import (
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/executor"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func executorWorkspaceRoot(environment executor.TaskEnvironment) string {
	if environment.WorkspaceRoot != "" {
		return environment.WorkspaceRoot
	}
	return environment.WorkDir
}

func cloneAssignment(value semanticir.Assignment) semanticir.Assignment {
	copy := make(semanticir.Assignment, len(value))
	for key, item := range value {
		copy[key] = item
	}
	return copy
}

func cloneBehavior(value semanticir.BehaviorRef) semanticir.BehaviorRef {
	value.Conditions = cloneAssignment(value.Conditions)
	value.Inputs = cloneInputs(value.Inputs)
	return value
}

func cloneInputs(value map[string]semanticir.Literal) map[string]semanticir.Literal {
	if value == nil {
		return nil
	}
	copy := make(map[string]semanticir.Literal, len(value))
	for name, literal := range value {
		copy[name] = literal
	}
	return copy
}

func cloneChoices(values []semanticir.BehaviorChoice) []semanticir.BehaviorChoice {
	result := make([]semanticir.BehaviorChoice, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Behavior = cloneBehavior(value.Behavior)
	}
	return result
}

func clonePlans(values []semanticir.EditPlan) []semanticir.EditPlan {
	result := make([]semanticir.EditPlan, len(values))
	copy(result, values)
	for index := range result {
		result[index].Edits = append([]semanticir.ByteRangeReplacement(nil), values[index].Edits...)
		for edit := range result[index].Edits {
			result[index].Edits[edit].ExpectedBytes = append([]byte(nil), values[index].Edits[edit].ExpectedBytes...)
			result[index].Edits[edit].Replacement = append([]byte(nil), values[index].Edits[edit].Replacement...)
		}
		result[index].Expected.Conditions = cloneAssignment(values[index].Expected.Conditions)
		result[index].Expected.OutcomeIDs = append([]string(nil), values[index].Expected.OutcomeIDs...)
		result[index].Expected.Choices = cloneChoices(values[index].Expected.Choices)
	}
	return result
}

func cloneCommandEvidence(value executor.CommandEvidence) executor.CommandEvidence {
	value.Command = append([]string(nil), value.Command...)
	return value
}

func equalAssignments(left, right semanticir.Assignment) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func equalChoices(left, right []semanticir.BehaviorChoice) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Behavior.OperationID != right[index].Behavior.OperationID ||
			left[index].OutcomeID != right[index].OutcomeID ||
			!equalAssignments(left[index].Behavior.Conditions, right[index].Behavior.Conditions) ||
			semanticir.BehaviorRefKey(left[index].Behavior) != semanticir.BehaviorRefKey(right[index].Behavior) {
			return false
		}
	}
	return true
}

func assignmentKey(assignment semanticir.Assignment) string {
	keys := make([]string, 0, len(assignment))
	for key := range assignment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(assignment[key])
		builder.WriteByte(';')
	}
	return builder.String()
}

func behaviorKey(operationID string, assignment semanticir.Assignment) string {
	return operationID + "|" + assignmentKey(assignment)
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
