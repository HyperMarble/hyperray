// Set comparison reports missing members before extra members.
// It never uses a count match as an equality result.
package jibcatalog

import "sort"

func requireExact(
	required map[string]struct{},
	actual map[string]struct{},
	missingCode string,
	extraCode string,
) error {
	if values := missing(required, actual); len(values) != 0 {
		return catalogError(missingCode, values...)
	}
	if values := missing(actual, required); len(values) != 0 {
		return catalogError(extraCode, values...)
	}
	return nil
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
