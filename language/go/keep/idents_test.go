// Purpose: the keep file takes the usual names when the package does not
// have them, else the first numbered variant it does not.
// Never:   gives two runs on the same package different names.
package keep

import "testing"

func TestAFreeNameIsKeptAndATakenOneGetsTheNextNumber(t *testing.T) {
	cases := []struct {
		taken map[string]bool
		want  string
	}{
		{map[string]bool{}, "hyperrayKeep"},
		{map[string]bool{"hyperrayKeep": true}, "hyperrayKeep1"},
		{map[string]bool{"hyperrayKeep": true, "hyperrayKeep1": true}, "hyperrayKeep2"},
	}
	for _, one := range cases {
		if got := freeIdent("hyperrayKeep", one.taken); got != one.want {
			t.Fatalf("taken %v: got %s, want %s", one.taken, got, one.want)
		}
	}
}

func TestATakenTestingNameGetsAnAliasedImport(t *testing.T) {
	idents := identsFor(map[string]bool{"testing": true})
	if idents.importLine() != `testing1 "testing"` {
		t.Fatalf("import line: %s", idents.importLine())
	}
	if identsFor(map[string]bool{}).importLine() != `"testing"` {
		t.Fatal("a free testing name must keep the plain import")
	}
}
