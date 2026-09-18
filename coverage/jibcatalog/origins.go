// Origin collection preserves the independent Sail traversal catalog.
// It rejects unnamed and duplicate JIB nodes.
package jibcatalog

func collectOrigins(values []Origin) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value.ID == "" {
			return nil, catalogError("empty_origin_id")
		}
		if value.Category == "" {
			return nil, catalogError("empty_origin_category", value.ID)
		}
		if value.Kind == "" {
			return nil, catalogError("empty_origin_kind", value.ID)
		}
		if _, exists := result[value.ID]; exists {
			return nil, catalogError("duplicate_origin", value.ID)
		}
		result[value.ID] = struct{}{}
	}
	return result, nil
}
