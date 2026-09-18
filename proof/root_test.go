// Root tests keep query reachability separate from whole-inventory coverage.
// They never let one query remove evidence for another root.
package proof_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/proof"
)

func TestQuerySpecificRoots(t *testing.T) {
	validated := validatedFixture(t)
	mainResult, err := proof.Check(validated, safeQuery("main-root"))
	if err != nil {
		t.Fatalf("main proof.Check() error = %v", err)
	}
	workerResult, err := proof.Check(validated, safeQuery("worker-root"))
	if err != nil {
		t.Fatalf("worker proof.Check() error = %v", err)
	}
	if mainResult.Verdict != proof.VerdictProved || workerResult.Verdict != proof.VerdictDisproved {
		t.Errorf("verdicts = %q, %q", mainResult.Verdict, workerResult.Verdict)
	}
	if !reflect.DeepEqual(mainResult.Coverage, workerResult.Coverage) {
		t.Errorf("coverage changed by roots: %#v != %#v", mainResult.Coverage, workerResult.Coverage)
	}
}

func TestNonQueryRootStillRequiresCoverage(t *testing.T) {
	request := loadFixture(t)
	request.Certificate.Environment = nil
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err == nil {
		t.Fatal("coverage.Check() error = nil")
	}
	result, proofErr := proof.Check(validated, safeQuery("main-root"))
	requireProofError(t, proofErr, "invalid_coverage")
	if result.Verdict != "" {
		t.Errorf("proof.Check().Verdict = %q", result.Verdict)
	}
}
