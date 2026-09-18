// Complete-certificate tests assert exact coverage decisions and counts.
// They never count eliminated operations or impossible roots as mapped.
package coverage_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
)

func TestCompleteCertificate(t *testing.T) {
	request := completeRequest(t)
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	got, err := validated.Report()
	if err != nil {
		t.Fatalf("ValidatedCoverage.Report() error = %v", err)
	}
	want := coverage.Report{
		Complete: true, RootedFunctions: 2, TotalFunctions: 2,
		MappedRoots: 2, ProvedImpossibleRoots: 0,
		CoveredRoots: 2, TotalRoots: 2, MappedOperations: 3,
		EliminatedOperations: 0, CoveredOperations: 3, TotalOperations: 3,
		ReconciledArtifacts: 5, TotalArtifacts: 5,
		ReconciledCompilerOutputs: 1, TotalCompilerOutputs: 1,
		ReconciledImageInstructions: 1, TotalImageInstructions: 1,
		ReconciledSemanticRules: 3, TotalSemanticRules: 3,
		ReconciledProvenanceEdges: 11, TotalProvenanceEdges: 11,
		ReconciledImpossibleProofs: 0, TotalImpossibleProofs: 0,
		ReconciledEliminationProofs: 0, TotalEliminationProofs: 0,
		ReconciledEliminationRecords: 0, TotalEliminationRecords: 0,
		MappedTransitions: 3, TotalTransitions: 3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Check() = %#v, want %#v", got, want)
	}
}

func TestOperationPublicValues(t *testing.T) {
	want := []string{"compiler", "synthetic", "environment", "mapped", "eliminated_with_proof"}
	got := []string{string(coverage.OperationCompiler), string(coverage.OperationSynthetic),
		string(coverage.OperationEnvironment), string(coverage.DispositionMapped),
		string(coverage.DispositionEliminated)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("public values = %v, want %v", got, want)
	}
}
