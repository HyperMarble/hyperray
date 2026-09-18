// Coverage errors identify the failed rule and its related references.
// They never convert invalid evidence into a coverage result.
package coverage

import "strings"

type Error struct {
	Code       string   `json:"code"`
	References []string `json:"references"`
	cause      error
}

func (failure *Error) Error() string {
	message := "coverage: " + failure.Code
	if len(failure.References) == 0 {
		return message
	}
	return message + ": " + strings.Join(failure.References, ", ")
}

func (failure *Error) Unwrap() error {
	return failure.cause
}

func coverageError(code string, references ...string) error {
	return &Error{Code: code, References: references}
}
