// Purpose: a main package built as a shared library, a plugin or a c
// archive is labelled as such in the record, from the mode go stamped or,
// for an archive, from the mode asked for.
// Never:   calls a library a program.
package goadapter_test

import (
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const cgoMain = "package main\n\nimport \"C\"\n\nfunc main() { println(1) }\n"

func TestABuildModeNamesTheKindOfFile(t *testing.T) {
	cgoOn(t)
	for mode, label := range map[string]string{"c-shared": "c shared library", "plugin": "plugin", "c-archive": "c archive"} {
		root, out := writeModule(t, map[string]string{"main.go": cgoMain})
		record := built(t, root, out, goadapter.Choice{Flags: []string{"-buildmode=" + mode}})
		if record.Artifacts[0].Kind != label {
			t.Fatalf("%s: labelled %q", mode, record.Artifacts[0].Kind)
		}
		if (mode == "c-archive") != (record.Artifacts[0].BuildInfo.GoVersion == "") {
			t.Fatalf("%s: stamp %+v", mode, record.Artifacts[0].BuildInfo)
		}
	}
}
