// External tests operate the complete public JIB-catalog API.
// They never depend on an instruction name or fixture-specific value.
package jibcatalog_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage/jibcatalog"
)

func completeCatalog() jibcatalog.Catalog {
	return jibcatalog.Catalog{
		Origins: []jibcatalog.Origin{
			{ID: "origin:first", Category: "instruction", Kind: "alpha"},
			{ID: "origin:second", Category: "type", Kind: "beta"},
		},
		Translations: []jibcatalog.Translation{
			{
				OriginID: "origin:first", RuleID: "rule:first",
				ProofObligationID: "proof:first", Disposition: jibcatalog.DispositionEmitted,
				RegionIDs: []string{"region:first"},
			},
			{
				OriginID: "origin:second", RuleID: "rule:second",
				ProofObligationID: "proof:second", Disposition: jibcatalog.DispositionErased,
			},
		},
		Regions: []jibcatalog.Region{{ID: "region:first"}},
	}
}

func TestCompleteCatalog(t *testing.T) {
	report, err := jibcatalog.Validate(completeCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete || report.Origins != 2 || report.Translations != 2 || report.Regions != 1 {
		t.Fatalf("wrong report: %+v", report)
	}
}
