// Purpose: the keep list names plain functions and methods of both receiver
// kinds, written with or without parentheses, and leaves out what Go
// cannot refer to by value.
// Never:   names a generic function or a method of a generic type.
package keep

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/language/go/module"
)

const kinds = `package kinds

type T struct{ n int }

func (t T) Value() int    { return t.n }
func (t *T) Pointer() int { return t.n }

func (t (T)) Paren() int      { return t.n }
func (t (*T)) ParenPtr() int  { return t.n }
func (t *(T)) StarParen() int { return t.n }

type G[X any] struct{ x X }

func (g G[X]) Get() X { return g.x }

func Generic[X any](x X) X { return x }

func plain() {}

func init() {}
`

const ownTestFile = "package kinds\n\nfunc helper() {}\n"
const externalTestFile = "package kinds_test\n\nfunc outside() {}\n"

func TestKeptNamesAreTheReferableFunctions(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"k.go": kinds, "k_test.go": ownTestFile, "x_test.go": externalTestFile} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg := module.Package{Dir: dir, GoFiles: []string{"k.go"}, TestGoFiles: []string{"k_test.go"}, XTestGoFiles: []string{"x_test.go"}}
	names, err := KeptNames(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if external, err := ExternalTestNames(pkg, nil); err != nil || !slices.Equal(external, []string{"outside"}) {
		t.Fatalf("external: %v %v", external, err)
	}
	want := []string{"T.Value", "(*T).Pointer", "T.Paren", "(*T).ParenPtr", "(*T).StarParen", "plain", "helper"}
	if !slices.Equal(names, want) {
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
	if _, err := WriteOverlay(dir, []module.Package{{Name: "r", Dir: dir, GoFiles: []string{"r.go"}}}, out, ""); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(out, "keep_0_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The package's own variable named nil is kept by its address, `&nil`.
	// Apart from that the file must not use a name the package redefined.
	rest := strings.ReplaceAll(string(text), "&nil", "")
	for _, builtin := range []string{"len(", "nil", "any"} {
		if strings.Contains(rest, builtin) {
			t.Fatalf("the keep file uses %q:\n%s", builtin, text)
		}
	}
}
