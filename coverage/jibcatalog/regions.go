// Region collection records the circuit builder output without inference.
// It rejects unnamed and duplicate regions.
package jibcatalog

func collectRegions(values []Region) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value.ID == "" {
			return nil, catalogError("empty_region_id")
		}
		if _, exists := result[value.ID]; exists {
			return nil, catalogError("duplicate_region", value.ID)
		}
		result[value.ID] = struct{}{}
	}
	return result, nil
}
