// This file defines observable model-validation failures.
// It never hides the identifiers that caused a failure.
package model

import (
	"sort"
	"strings"
)

// ValidationError identifies one invalid graph condition.
type ValidationError struct {
	Code       string   `json:"code"`
	References []string `json:"references"`
}

// Error returns a stable description of the validation failure.
func (problem ValidationError) Error() string {
	message := "model validation failed: " + problem.Code
	if len(problem.References) == 0 {
		return message
	}
	references := sortedReferences(problem.References)
	return message + " [" + strings.Join(references, ", ") + "]"
}

func validationError(code string, references ...string) error {
	return &ValidationError{Code: code, References: sortedReferences(references)}
}

func sortedReferences(references []string) []string {
	result := append([]string(nil), references...)
	sort.Strings(result)
	return result
}
