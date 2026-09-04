// Diagnostic mutation tests reject missing, changed, and unsupported dispositions.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintCoverageRejectsDiagnosticMismatch(t *testing.T) {
	cases := []struct {
		name   string
		change reportMutation
	}{
		{name: "missing", change: removeDiagnosticDisposition},
		{name: "message", change: changeDiagnosticMessage},
		{name: "kind", change: changeDiagnosticKind},
		{name: "disposition", change: changeDiagnosticDisposition},
		{name: "evidence", change: changeDiagnosticEvidence},
		{name: "extra", change: addUnexpectedDisposition},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.name, func(t *testing.T) {
			assertDiagnosticMutationRejected(t, testCase.change)
		})
	}
}

func assertDiagnosticMutationRejected(t *testing.T, change reportMutation) {
	t.Helper()
	instructions := []machine.Instruction{{Address: 2, Bytes: []byte{0xab, 0xab, 0xab, 0xab}}}
	report, err := footprintEngine(t).TraceInstructions(t.Context(), footprintRequest(t, instructions, 4096))
	if err != nil {
		t.Fatalf("TraceInstructions() error = %v", err)
	}
	report = cloneFootprintReport(report)
	report.Instructions[0].Dispositions = append([]isla.DiagnosticDisposition(nil), report.Instructions[0].Dispositions...)
	change(&report)
	assertCoverageMismatch(t, instructions, report)
}

func removeDiagnosticDisposition(report *isla.FootprintReport) {
	report.Instructions[0].Dispositions = nil
}

func changeDiagnosticMessage(report *isla.FootprintReport) {
	report.Instructions[0].Dispositions[0].Message = "changed"
}

func changeDiagnosticKind(report *isla.FootprintReport) {
	report.Instructions[0].Dispositions[0].Kind = "changed"
}

func changeDiagnosticDisposition(report *isla.FootprintReport) {
	report.Instructions[0].Dispositions[0].Disposition = "changed"
}

func changeDiagnosticEvidence(report *isla.FootprintReport) {
	report.Instructions[0].Dispositions[0].EvidenceDigest = "changed"
}

func addUnexpectedDisposition(report *isla.FootprintReport) {
	report.Instructions[0].Diagnostics = ""
}
