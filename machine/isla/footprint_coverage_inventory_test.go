// Inventory mutation tests reject every missing, extra, changed, or duplicate record.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

type reportMutation func(*isla.FootprintReport)

func TestFootprintCoverageRejectsInventoryMismatch(t *testing.T) {
	cases := []struct {
		name   string
		change reportMutation
	}{
		{name: "empty", change: emptyTraceInventory},
		{name: "missing", change: removeTrace},
		{name: "extra", change: addExtraTrace},
		{name: "duplicate", change: duplicateTrace},
		{name: "changed", change: changeTraceEncoding},
		{name: "unproved", change: removeTraceProof},
		{name: "digest", change: invalidateTraceDigest},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.name, func(t *testing.T) {
			assertInventoryMutationRejected(t, testCase.change)
		})
	}
}

func assertInventoryMutationRejected(t *testing.T, change reportMutation) {
	t.Helper()
	instructions, original := coverageFixture(t)
	report := cloneFootprintReport(original)
	change(&report)
	assertCoverageMismatch(t, instructions, report)
}

func emptyTraceInventory(report *isla.FootprintReport) {
	report.Instructions = nil
}

func removeTrace(report *isla.FootprintReport) {
	report.Instructions = report.Instructions[:1]
}

func addExtraTrace(report *isla.FootprintReport) {
	extra := report.Instructions[0]
	extra.Address = 0x2000
	report.Instructions = append(report.Instructions, extra)
}

func duplicateTrace(report *isla.FootprintReport) {
	report.Instructions = append(report.Instructions, report.Instructions[0])
}

func changeTraceEncoding(report *isla.FootprintReport) {
	report.Instructions[0].Encoding = "ffff"
}

func removeTraceProof(report *isla.FootprintReport) {
	report.Instructions[0].TraceCount = 0
}

func invalidateTraceDigest(report *isla.FootprintReport) {
	report.Instructions[0].OutputDigest = "bad"
}
