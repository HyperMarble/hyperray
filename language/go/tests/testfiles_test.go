// Purpose: a function in the package's own test file, and one in its
// external _test package, each have their own machine code in the test
// program even when nothing calls them.
// Never:   lets the linker drop test code that was compiled.
package goadapter_test

import (
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const ownHelper = "package lib\n\nfunc unusedHelper() int { return 9 }\n"
const externalHelper = "package lib_test\n\nfunc unusedExternal() int { return 8 }\n"

func TestFunctionsInTestFilesHaveMachineCode(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": library, "lib/own_test.go": ownHelper, "lib/ext_test.go": externalHelper,
	})
	record := built(t, root, out, goadapter.Choice{})
	names := symbols(t, record.Artifacts[0].File.Path)
	for _, want := range []string{"lib.unusedHelper", "lib_test.unusedExternal", "lib.unused"} {
		if !hasSymbol(names, want) {
			t.Fatalf("%s is missing from the test program", want)
		}
	}
}
