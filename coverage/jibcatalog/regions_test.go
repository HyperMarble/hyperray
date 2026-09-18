// Region tests reject unnamed, duplicate, missing, and extra ownership.
// They never accept a circuit region by count alone.
package jibcatalog_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage/jibcatalog"
)

func TestRegionFailures(t *testing.T) {
	tests := []struct {
		name   string
		change func(*jibcatalog.Catalog)
		code   string
	}{
		{"empty region", emptyRegion, "empty_region_id"},
		{"duplicate region", duplicateRegion, "duplicate_region"},
		{"unmapped region", addUnmappedRegion, "missing_region_mapping"},
		{"unknown mapped region", mapUnknownRegion, "extra_region_mapping"},
		{"shared region", shareRegion, "duplicate_region_mapping"},
		{"empty mapped region", mapEmptyRegion, "empty_region_mapping"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			value := completeCatalog()
			testCase.change(&value)
			requireCode(t, value, testCase.code)
		})
	}
}

func emptyRegion(value *jibcatalog.Catalog) { value.Regions[0].ID = "" }
func duplicateRegion(value *jibcatalog.Catalog) {
	value.Regions = append(value.Regions, value.Regions[0])
}
func addUnmappedRegion(value *jibcatalog.Catalog) {
	value.Regions = append(value.Regions, jibcatalog.Region{ID: "region:second"})
}
func mapUnknownRegion(value *jibcatalog.Catalog) {
	value.Translations[0].RegionIDs = append(value.Translations[0].RegionIDs, "region:unknown")
}
func shareRegion(value *jibcatalog.Catalog) {
	value.Translations[1].Disposition = jibcatalog.DispositionEmitted
	value.Translations[1].RegionIDs = []string{"region:first"}
}
func mapEmptyRegion(value *jibcatalog.Catalog) { value.Translations[0].RegionIDs[0] = "" }
