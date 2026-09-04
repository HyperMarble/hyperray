// This file gives each rejected ELF an observable cause.
// It must preserve a parser error when the standard parser rejects input.
package machine

import "fmt"

// Rejection identifies the exact profile rule that rejected an ELF.
type Rejection struct {
	Code   RejectionCode
	Detail string
	Cause  error
}

func (rejection *Rejection) Error() string {
	message := string(rejection.Code) + ": " + rejection.Detail
	if rejection.Cause == nil {
		return message
	}
	return message + ": " + rejection.Cause.Error()
}

func (rejection *Rejection) Unwrap() error {
	return rejection.Cause
}

func reject(code RejectionCode, detail string, cause error) error {
	problem := &Rejection{Code: code, Detail: detail, Cause: cause}
	return fmt.Errorf("load %s: %w", ProfileName, problem)
}
