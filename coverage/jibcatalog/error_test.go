// Public error tests expose stable machine-readable error codes.
// They never require callers to parse an unstructured cause.
package jibcatalog_test

import (
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/coverage/jibcatalog"
)

func requireCode(t *testing.T, value jibcatalog.Catalog, code string) {
	t.Helper()
	_, err := jibcatalog.Validate(value)
	var failure *jibcatalog.Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("wanted %q, got %v", code, err)
	}
}

func TestPublicErrorText(t *testing.T) {
	plain := (&jibcatalog.Error{Code: "plain"}).Error()
	if plain != "JIB circuit catalog: plain" {
		t.Fatalf("plain Error() = %q", plain)
	}
	failure := &jibcatalog.Error{Code: "missing", References: []string{"alpha", "beta"}}
	want := "JIB circuit catalog: missing: alpha, beta"
	if failure.Error() != want {
		t.Fatalf("Error() = %q, want %q", failure.Error(), want)
	}
}
