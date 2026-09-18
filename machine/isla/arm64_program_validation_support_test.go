// ARM64 validation helpers keep rejection assertions separate from cases.
package isla_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func assertARM64Error(t *testing.T, err error, code isla.ErrorCode, subject string, detail string) {
	t.Helper()
	var failure *isla.Error
	if !errors.As(err, &failure) || failure.Code != code || failure.Subject != subject || !strings.Contains(failure.Detail, detail) {
		t.Fatalf("error = %v, want %s/%s containing %q", err, code, subject, detail)
	}
}

func assertZeroARM64Program(t *testing.T, program isla.Program) {
	t.Helper()
	if program.Content() != nil || program.ImageDigest() != "" || program.Evidence().Profile != "" {
		t.Fatalf("failed build returned non-zero program: %#v", program.Evidence())
	}
}
