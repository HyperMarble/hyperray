// External tests measure every public success and failure result.
// Each incomplete relation must return its exact public error.
package sailcatalog_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/coverage/sailcatalog"
)

func TestCompleteCatalog(t *testing.T) {
	result, err := sailcatalog.Validate(catalogJSON(`["A","B"]`, `["B","A"]`, `["A","B"]`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Complete || result.Families != 2 {
		t.Fatalf("wrong report: %+v", result)
	}
}

func TestPublicErrorMessages(t *testing.T) {
	plain := (&sailcatalog.Error{Code: "plain"}).Error()
	if plain != "sail catalog: plain" {
		t.Fatalf("wrong plain error: %q", plain)
	}
	withReferences := (&sailcatalog.Error{
		Code: "failure", References: []string{"A", "B"},
	}).Error()
	if withReferences != "sail catalog: failure: A, B" {
		t.Fatalf("wrong referenced error: %q", withReferences)
	}
}

func TestCatalogFailures(t *testing.T) {
	tests := []struct{ name, declared, decoded, executed, code string }{
		{"invalid JSON", `[`, `[]`, `[]`, "invalid_json"},
		{"unknown field", `["A"],"unknown":true`, `["A"]`, `["A"]`, "invalid_json"},
		{"trailing JSON", `["A"]`, `["A"]`, `["A"]} {`, "invalid_json"},
		{"empty", `[]`, `[]`, `[]`, "empty_instruction_catalog"},
		{"empty name", `[""]`, `[""]`, `[""]`, "empty_name"},
		{"duplicate declaration", `["A","A"]`, `["A"]`, `["A"]`, "duplicate_name"},
		{"duplicate decode", `["A"]`, `["A","A"]`, `["A"]`, "duplicate_name"},
		{"duplicate execute", `["A"]`, `["A"]`, `["A","A"]`, "duplicate_name"},
		{"missing decode", `["A","B"]`, `["A"]`, `["A","B"]`, "missing_decode_clause"},
		{"extra decode", `["A"]`, `["A","B"]`, `["A"]`, "extra_decode_clause"},
		{"missing execute", `["A","B"]`, `["A","B"]`, `["A"]`, "missing_execute_clause"},
		{"extra execute", `["A"]`, `["A"]`, `["A","B"]`, "extra_execute_clause"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireCode(t, catalogJSON(test.declared, test.decoded, test.executed), test.code)
		})
	}
}

func catalogJSON(declared string, decoded string, executed string) []byte {
	value := `{"instruction_declarations":` + declared +
		`,"decode_clauses":` + decoded + `,"execute_clauses":` + executed + `}`
	return []byte(value)
}

func requireCode(t *testing.T, data []byte, code string) {
	t.Helper()
	_, err := sailcatalog.Validate(data)
	var failure *sailcatalog.Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("wanted %q, got %v", code, err)
	}
}
