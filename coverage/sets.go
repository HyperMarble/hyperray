// Set operations make identity and reconciliation checks deterministic.
// They never accept an empty, duplicate, or unknown identifier.
package coverage

import "sort"

type idSet map[string]struct{}

type evidenceField struct {
	name  string
	value string
}

func requireEvidence(fields ...evidenceField) error {
	for _, field := range fields {
		if field.value == "" {
			return coverageError("empty_field", field.name)
		}
	}
	return nil
}

func uniqueIDSet(values []string, catalog string) (idSet, error) {
	ordered := append([]string(nil), values...)
	sort.Strings(ordered)
	ids := make(idSet, len(ordered))
	for _, id := range ordered {
		if id == "" {
			return nil, coverageError("empty_id", catalog)
		}
		if !validIdentifier(id) {
			return nil, coverageError("invalid_identifier", catalog, id)
		}
		if _, exists := ids[id]; exists {
			return nil, coverageError("duplicate_id", catalog, id)
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

func sortedMissingIDs(required idSet, actual idSet) []string {
	missing := make([]string, 0)
	for id := range required {
		if _, exists := actual[id]; !exists {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedIDs(ids idSet) []string {
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func equalIDSets(left idSet, right idSet) bool {
	return len(left) == len(right) && len(sortedMissingIDs(left, right)) == 0
}
