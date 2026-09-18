// Error identifies a proof-engine input or coverage problem.
// It never hides the lower-level cause.
package proof

import "strings"

type Error struct {
	Code       string   `json:"code"`
	References []string `json:"references"`
	cause      error
}

func (failure *Error) Error() string {
	message := "proof: " + failure.Code
	if len(failure.References) == 0 {
		return message
	}
	return message + ": " + strings.Join(failure.References, ", ")
}

func (failure *Error) Unwrap() error {
	return failure.cause
}

func proofError(code string, references ...string) error {
	return &Error{Code: code, References: references}
}

func coverageFailure(err error) error {
	return &Error{Code: "invalid_coverage", References: []string{err.Error()}, cause: err}
}
