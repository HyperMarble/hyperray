// Test origin data gives each neutral constructor kind one occurrence.
// It never copies an architecture-specific generated artifact.
package main

import "fmt"

func validOrigins(value catalog) []origin {
	result := make([]origin, 0)
	for _, current := range catalogCategories(value) {
		for _, kind := range current.values {
			result = append(result, origin{
				ID: fmt.Sprintf("origin:%d", len(result)), Category: current.key, Kind: kind,
			})
		}
	}
	return result
}
