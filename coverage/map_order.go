// Map ordering makes validation independent from Go map iteration.
// It never exposes an arbitrary first failure.
package coverage

import "sort"

func sortedOperationIDs(values map[string]Operation) []string {
	result := make([]string, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func sortedArtifactIDs(values map[string]Artifact) []string {
	result := make([]string, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
