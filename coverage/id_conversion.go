// Identifier conversion supports exact comparisons after prior validation.
// It never performs validation or invents missing identifiers.
package coverage

func operationIDSet(values map[string]Operation) idSet {
	ids := make(idSet, len(values))
	for id := range values {
		ids[id] = struct{}{}
	}
	return ids
}

func stringIDSet(values []string) idSet {
	ids := make(idSet, len(values))
	for _, id := range values {
		ids[id] = struct{}{}
	}
	return ids
}

func eliminationIDSet(values map[string]EliminationRecord) idSet {
	ids := make(idSet, len(values))
	for id := range values {
		ids[id] = struct{}{}
	}
	return ids
}
