// Catalog errors identify the exact incomplete relation.
// They never return a completed coverage report.
package jibcatalog

import "strings"

type Error struct {
	Code       string
	References []string
}

func (failure *Error) Error() string {
	message := "JIB circuit catalog: " + failure.Code
	if len(failure.References) == 0 {
		return message
	}
	return message + ": " + strings.Join(failure.References, ", ")
}

func catalogError(code string, references ...string) error {
	return &Error{Code: code, References: references}
}
