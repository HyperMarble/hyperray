// Package isla connects Hyperray to one identified Isla executable.
// It must return engine errors instead of proof results after tool failures.
package isla

import "fmt"

// ErrorCode identifies one machine-integration error class.
type ErrorCode string

const (
	InvalidInput     ErrorCode = "invalid_input"
	ArtifactChanged  ErrorCode = "artifact_changed"
	ToolNotFound     ErrorCode = "tool_not_found"
	ToolIdentityFail ErrorCode = "tool_identity_error"
	ToolChanged      ErrorCode = "tool_changed"
	ReleaseMismatch  ErrorCode = "release_mismatch"
	CoverageMismatch ErrorCode = "coverage_mismatch"
	ProcessFail      ErrorCode = "process_error"
	ResourceLimit    ErrorCode = "resource_limit"
	ResultError      ErrorCode = "result_error"
	ProtocolError    ErrorCode = "protocol_error"
)

// Error gives callers a stable code and the exact failure context.
type Error struct {
	Code    ErrorCode
	Subject string
	Detail  string
}

func (failure *Error) Error() string {
	return fmt.Sprintf("isla %s: %s: %s", failure.Code, failure.Subject, failure.Detail)
}

func engineError(code ErrorCode, subject string, detail string) error {
	return &Error{Code: code, Subject: subject, Detail: detail}
}
