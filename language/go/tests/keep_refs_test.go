// Purpose: every function Go lets a program refer to by value keeps its
// machine code, including a library function named main and methods named
// init or main; the two things the keep step skips are exactly the two
// things the compiler refuses to refer to.
// Never:   skips a function by its name when Go would accept the reference.
package goadapter_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const namedLikeEntryPoints = `package lib

type T struct{}

func main() int { return 11 }

func (T) init() int { return 22 }

func (*T) main() int { return 33 }

func Double(a int) int { return a * 2 }

func init() {}

func _() {}

func (T) _() {}
`

func TestFunctionsAndMethodsNamedMainOrInitKeepTheirMachineCode(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": namedLikeEntryPoints})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.NotBuilt) > 0 {
		t.Fatalf("not built: %+v", record.NotBuilt)
	}
	names := symbols(t, record.Artifacts[0].File.Path)
	for _, want := range []string{"lib.main", "lib.T.init", "lib.(*T).main"} {
		if !hasSymbol(names, want) {
			t.Fatalf("%s is missing from the test program", want)
		}
	}
}

// goRefuses is true when the compiler rejects a test file that refers to
// the declaration by value: the owner's answer, asked live.
func goRefuses(t *testing.T, declaration, reference string) bool {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":    sampleModule,
		"a.go":      "package lib\n\ntype T struct{}\n\n" + declaration + "\n",
		"a_test.go": "package lib\n\nimport \"testing\"\n\nvar keep = []interface{}{" + reference + "}\n\nfunc TestKeep(t *testing.T) { t.Log(keep) }\n",
	}
	for name, text := range files {
		writeText(t, filepath.Join(dir, name), text)
	}
	command := exec.Command("go", "test", "-c", "-o", os.DevNull, ".")
	command.Dir = dir
	return command.Run() != nil
}

func TestTheNamesTheKeepStepSkipsAreTheOnesGoRefusesToReferTo(t *testing.T) {
	refused := [][2]string{{"func init() {}", "init"}, {"func _() {}", "_"}, {"func (T) _() {}", "T._"}}
	for _, one := range refused {
		if !goRefuses(t, one[0], one[1]) {
			t.Fatalf("Go now accepts a reference to %q: the skip rule is out of date", one[1])
		}
	}
	accepted := [][2]string{{"func main() {}", "main"}, {"func (T) init() {}", "T.init"}, {"func (T) main() {}", "T.main"}}
	for _, one := range accepted {
		if goRefuses(t, one[0], one[1]) {
			t.Fatalf("Go refuses a reference to %q: the keep step should skip it", one[1])
		}
	}
}
