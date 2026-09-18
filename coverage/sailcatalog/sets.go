// Catalog set operations keep equality failures stable and exact.
// They reject empty names and duplicate owners.
package sailcatalog

import "sort"

func catalogSet(values []string, name string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return nil, catalogError("empty_name", name)
		}
		if _, exists := result[value]; exists {
			return nil, catalogError("duplicate_name", name, value)
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func missing(required map[string]struct{}, actual map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range required {
		if _, exists := actual[value]; !exists {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
