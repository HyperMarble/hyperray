// Parser test support builds complete text and checks typed protocol failures.
// It keeps malformed cases visible in their calling tests.
package isla

import (
	"errors"
	"testing"
)

func semanticTestOutput(threads string, assertion string, footprints string) string {
	return threads + "\nFinal Assertion:\n" + assertion + "\nMemory:test\n" + footprints
}

func requireSemanticError(t *testing.T, output string, code ErrorCode) {
	t.Helper()
	summary, err := parseSemanticOutput(output)
	if err == nil {
		t.Fatalf("parseSemanticOutput() = %#v, nil error", summary)
	}
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Errorf("error = %v, want code %q", err, code)
	}
}
