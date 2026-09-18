// Translation tests reject missing, duplicate, and unknown JIB mappings.
// They never infer a lowerer decision from a matching count.
package jibcatalog_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/coverage/jibcatalog"
)

func TestTranslationFailures(t *testing.T) {
	tests := []struct {
		name   string
		change func(*jibcatalog.Catalog)
		code   string
	}{
		{"missing", removeSecondTranslation, "missing_translation"},
		{"duplicate", duplicateFirstTranslation, "duplicate_translation"},
		{"unknown", unknownTranslation, "unknown_origin"},
		{"empty origin", emptyTranslationOrigin, "empty_translation_field"},
		{"empty rule", emptyTranslationRule, "empty_translation_field"},
		{"empty proof", emptyTranslationProof, "empty_translation_field"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			value := completeCatalog()
			testCase.change(&value)
			requireCode(t, value, testCase.code)
		})
	}
}

func removeSecondTranslation(value *jibcatalog.Catalog) {
	value.Translations = value.Translations[:1]
}

func duplicateFirstTranslation(value *jibcatalog.Catalog) {
	value.Translations[1] = value.Translations[0]
}

func unknownTranslation(value *jibcatalog.Catalog) {
	value.Translations[0].OriginID = "origin:unknown"
}

func emptyTranslationOrigin(value *jibcatalog.Catalog) { value.Translations[0].OriginID = "" }
func emptyTranslationRule(value *jibcatalog.Catalog)   { value.Translations[0].RuleID = "" }
func emptyTranslationProof(value *jibcatalog.Catalog)  { value.Translations[0].ProofObligationID = "" }
