// Purpose: a nested module is built as its own and sits under the parent's
// record; one under a folder Go ignores is listed by hash and not built;
// a nested module's own nested module is found.
// Never:   builds a module Go's own rules keep out of ./...
package goadapter_test

import (
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const innerModule = "module example.com/inner\n\ngo 1.25\n"
const innerSource = "package x\n\nfunc X() int { return 1 }\n"

func TestANestedModuleIsBuiltAsItsOwn(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": library, "inner/go.mod": innerModule, "inner/x/x.go": innerSource,
	})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Nested) != 1 || filepath.Base(record.Nested[0].Dir) != "inner" {
		t.Fatalf("nested: %+v", record.Nested)
	}
	inner := record.Nested[0].Record
	if len(inner.Artifacts) != 1 || !hasSymbol(symbols(t, inner.Artifacts[0].File.Path), "x.X") {
		t.Fatalf("inner artifacts: %+v", inner.Artifacts)
	}
}

func TestAModuleUnderAnIgnoredFolderIsListedNotBuilt(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go":        library,
		"testdata/m/go.mod": innerModule, "testdata/m/m.go": innerSource,
		"_skip/m/go.mod": innerModule, "_skip/m/m.go": innerSource,
		".hidden/m/go.mod": innerModule, ".hidden/m/m.go": innerSource,
	})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Nested) != 0 || len(record.IgnoredModules) != 3 {
		t.Fatalf("nested %d ignored %d", len(record.Nested), len(record.IgnoredModules))
	}
	for _, ignored := range record.IgnoredModules {
		if filepath.Base(ignored.Path) != "go.mod" || ignored.Sha256 == "" {
			t.Fatalf("ignored module: %+v", ignored)
		}
	}
}

func TestANestedModuleInsideANestedModuleIsFound(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": library, "inner/go.mod": innerModule, "inner/x/x.go": innerSource,
		"inner/deeper/go.mod": "module example.com/deeper\n\ngo 1.25\n", "inner/deeper/y/y.go": "package y\n\nfunc Y() int { return 2 }\n",
	})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Nested) != 1 || len(record.Nested[0].Record.Nested) != 1 {
		t.Fatalf("nested: %+v", record.Nested)
	}
}
