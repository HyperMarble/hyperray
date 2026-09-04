// Header tests reject each result class outside Allowed and Forbidden.
// They must keep Isla diagnostics in a typed result error.
package isla

import (
	"errors"
	"testing"
)

func TestResultHeaderErrors(t *testing.T) {
	cases := []struct {
		name        string
		lines       []string
		diagnostics string
		code        ErrorCode
	}{
		{name: "empty", lines: nil, code: ProtocolError},
		{name: "malformed", lines: []string{"bad"}, code: ProtocolError},
		{name: "unknown", lines: []string{"Test q Unknown"}, code: ProtocolError},
		{name: "tool", lines: []string{"Test q Error"}, diagnostics: "stopped", code: ResultError},
		{name: "tool-empty", lines: []string{"Test q Error"}, code: ResultError},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.name, func(t *testing.T) {
			name, status, err := resultHeader(testCase.lines, testCase.diagnostics)
			if err == nil {
				t.Errorf("resultHeader() = %q, %q, nil", name, status)
			}
			assertInternalCode(t, err, testCase.code)
		})
	}
}

func assertInternalCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v", err)
	}
	if failure.Code != code {
		t.Errorf("error code = %q, want %q", failure.Code, code)
	}
}
