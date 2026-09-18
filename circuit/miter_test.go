// Miter tests measure stable public bytes and identities for equal inputs.
// They never compare an implementation-private syntax tree.
package circuit_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestPublicMiterIsDeterministic(t *testing.T) {
	relation := balanceRelation(t, 8, "unsigned_less_than")
	first, firstError := circuit.BuildMiter(relation, relation)
	second, secondError := circuit.BuildMiter(relation, relation)
	if firstError != nil || secondError != nil {
		t.Fatalf("BuildMiter() errors = %v, %v", firstError, secondError)
	}
	if first.SMT2() != second.SMT2() || first.Digest() != second.Digest() {
		t.Error("equal relations produced different miter artifacts")
	}
	if !strings.HasPrefix(first.Digest(), "sha256:") {
		t.Errorf("Miter.Digest() = %q", first.Digest())
	}
	if !strings.Contains(first.SMT2(), "(assert (or ") {
		t.Errorf("Miter.SMT2() has no multi-output difference: %s", first.SMT2())
	}
}
