// Purpose: the adapter builds what the compiler accepts; vet's opinions do
// not block it, and a package made only of test files gets its test program.
// Never:   lets a lint stop a build the compiler accepts.
package goadapter_test

import (
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const vetWouldComplain = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printf(\"%d\\n\", \"not a number\") }\n"

const testOnly = "package only\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n"

func TestVetsOpinionDoesNotBlockTheBuild(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": vetWouldComplain})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 2 {
		t.Fatalf("artifacts: %+v", record.Artifacts)
	}
}

func TestATestOnlyPackageGetsItsTestProgramOnly(t *testing.T) {
	root, out := writeModule(t, map[string]string{"only/only_test.go": testOnly})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 1 || record.Artifacts[0].Kind != "test program" {
		t.Fatalf("artifacts: %+v", record.Artifacts)
	}
}
