// Catalog errors name the exact set relation that failed.
// They never turn an incomplete catalog into a coverage report.
package sailcatalog

import "strings"

type Error struct {
	Code       string
	References []string
}

func (failure *Error) Error() string {
	message := "sail catalog: " + failure.Code
	if len(failure.References) == 0 {
		return message
	}
	return message + ": " + strings.Join(failure.References, ", ")
}

func catalogError(code string, references ...string) error {
	return &Error{Code: code, References: references}
}
