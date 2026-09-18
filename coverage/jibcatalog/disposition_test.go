// Disposition tests make circuit output and proved erasure exclusive.
// They never permit an implicit no-output translation.
package jibcatalog_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage/jibcatalog"
)

func TestDispositionFailures(t *testing.T) {
	tests := []struct {
		name   string
		change func(*jibcatalog.Catalog)
		code   string
	}{
		{"unknown", setUnknownDisposition, "invalid_disposition"},
		{"emitted without region", removeEmittedRegion, "emitted_without_region"},
		{"erased with region", addErasedRegion, "erased_with_region"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			value := completeCatalog()
			testCase.change(&value)
			requireCode(t, value, testCase.code)
		})
	}
}

func setUnknownDisposition(value *jibcatalog.Catalog) {
	value.Translations[0].Disposition = "unknown"
}

func removeEmittedRegion(value *jibcatalog.Catalog) {
	value.Translations[0].RegionIDs = nil
}

func addErasedRegion(value *jibcatalog.Catalog) {
	value.Translations[1].RegionIDs = []string{"region:second"}
}
