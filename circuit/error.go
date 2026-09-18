// Package circuit builds bit-vector equivalence questions for proposal tools.
// It never converts an unvalidated solver result into a proof certificate.
package circuit

import "strings"

// EngineError reports one exact failure in circuit construction or proposal.
type EngineError struct {
	Code       string
	References []string
}

func (problem *EngineError) Error() string {
	return problem.Code + ": " + strings.Join(problem.References, ", ")
}

func engineError(code string, references ...string) error {
	return &EngineError{Code: code, References: append([]string(nil), references...)}
}
