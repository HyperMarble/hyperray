// Purpose: the keep list names plain functions and methods of both receiver
// kinds, and leaves out what Go cannot refer to by value.
// Never:   names a generic function or a method of a generic type.
package keep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/language/go/module"
)

const kinds = `package kinds

type T struct{ n int }

func (t T) Value() int    { return t.n }
func (t *T) Pointer() int { return t.n }

type G[X any] struct{ x X }

func (g G[X]) Get() X { return g.x }

func Generic[X any](x X) X { return x }

func plain() {}

func init() {}
`

func TestKeptNamesAreTheReferableFunctions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "k.go"), []byte(kinds), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := KeptNames(module.Package{Dir: dir, GoFiles: []string{"k.go"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"T.Value", "(*T).Pointer", "plain"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Fatalf("got %v, want %v", names, want)
	}
}

const redefinesBuiltins = "package r\n\nconst len = 20\n\nvar nil = 1\n\ntype any int\n\nfunc f() int { return len + nil }\n"

func TestAPackageThatRedefinesBuiltinsStillGetsItsKeepFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "r.go"), []byte(redefinesBuiltins), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if _, err := WriteOverlay([]module.Package{{Name: "r", Dir: dir, GoFiles: []string{"r.go"}}}, out, ""); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(out, "keep_0_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, builtin := range []string{"len(", "nil", "any"} {
		if strings.Contains(string(text), builtin) {
			t.Fatalf("the keep file uses %q:\n%s", builtin, text)
		}
	}
}
