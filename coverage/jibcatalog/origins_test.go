// Origin tests cover missing names, kinds, categories, and owners.
// They never use architecture-specific constructor names.
package jibcatalog_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage/jibcatalog"
)

func TestOriginFailures(t *testing.T) {
	tests := []struct {
		name   string
		change func(*jibcatalog.Catalog)
		code   string
	}{
		{"empty catalog", func(value *jibcatalog.Catalog) { value.Origins = nil }, "empty_origin_catalog"},
		{"empty id", func(value *jibcatalog.Catalog) { value.Origins[0].ID = "" }, "empty_origin_id"},
		{"empty category", func(value *jibcatalog.Catalog) { value.Origins[0].Category = "" }, "empty_origin_category"},
		{"empty kind", func(value *jibcatalog.Catalog) { value.Origins[0].Kind = "" }, "empty_origin_kind"},
		{"duplicate", func(value *jibcatalog.Catalog) { value.Origins[1].ID = value.Origins[0].ID }, "duplicate_origin"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			value := completeCatalog()
			testCase.change(&value)
			requireCode(t, value, testCase.code)
		})
	}
}
