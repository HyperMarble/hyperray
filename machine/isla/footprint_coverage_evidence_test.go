// Evidence mutation tests reject incomplete release identities and limits.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintCoverageRejectsEvidenceMismatch(t *testing.T) {
	cases := []struct {
		name   string
		change reportMutation
	}{
		{name: "identity", change: removeEvidenceIdentity},
		{name: "digest", change: invalidateEvidenceDigest},
		{name: "limit", change: removeEvidenceLimit},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.name, func(t *testing.T) {
			assertEvidenceMutationRejected(t, testCase.change)
		})
	}
}

func assertEvidenceMutationRejected(t *testing.T, change reportMutation) {
	t.Helper()
	instructions, original := coverageFixture(t)
	report := cloneFootprintReport(original)
	change(&report)
	assertCoverageMismatch(t, instructions, report)
}

func removeEvidenceIdentity(report *isla.FootprintReport) {
	report.Evidence.ReleaseID = ""
}

func invalidateEvidenceDigest(report *isla.FootprintReport) {
	report.Evidence.ManifestDigest = "bad"
}

func removeEvidenceLimit(report *isla.FootprintReport) {
	report.Evidence.ThreadLimit = 0
}
