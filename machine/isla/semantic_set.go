// Encoding-set helpers perform exact bidirectional coverage comparisons.
// Sorted copies make the accepted evidence stable and observable.
package isla

import "sort"

func equalEncodingSets(left map[string]struct{}, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for encoding := range left {
		if _, exists := right[encoding]; !exists {
			return false
		}
	}
	return true
}

func mergeEncodingSet(target map[string]struct{}, source map[string]struct{}) {
	for encoding := range source {
		target[encoding] = struct{}{}
	}
}

func mergeInstructionSet(target map[SemanticInstruction]struct{}, source map[SemanticInstruction]struct{}) {
	for instruction := range source {
		target[instruction] = struct{}{}
	}
}

func sortedEncodings(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedSemanticInstructions(values map[SemanticInstruction]struct{}) []SemanticInstruction {
	result := make([]SemanticInstruction, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(left int, right int) bool {
		if result[left].Address == result[right].Address {
			return result[left].Encoding < result[right].Encoding
		}
		return result[left].Address < result[right].Address
	})
	return result
}
