// Error tests expose the stable code and exact context to external callers.
// They must not hide the subject or detail.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicErrorText(t *testing.T) {
	failure := &isla.Error{Code: isla.ProtocolError, Subject: "result", Detail: "bad header"}
	want := "isla protocol_error: result: bad header"
	if failure.Error() != want {
		t.Errorf("Error() = %q, want %q", failure.Error(), want)
	}
}
