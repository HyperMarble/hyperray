// Purpose: when a flag the project builds with lets the program link but
// forbids linking a test program, the record keeps the program and lists
// the test program as not built with Go's words, instead of refusing.
// Never:   refuses a module whose own build succeeds.
package goadapter_test

import (
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const moreStackMain = "package main\n\nfunc mayMoreStack() {}\n\nfunc main() { println(1) }\n"

func TestATestProgramTheFlagsForbidIsRecordedNotRefused(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": moreStackMain})
	record := built(t, root, out, goadapter.Choice{Flags: []string{"-gcflags=-d=maymorestack=main.mayMoreStack"}})
	if len(record.Artifacts) != 1 || record.Artifacts[0].Kind != "program" {
		t.Fatalf("artifacts: %+v", record.Artifacts)
	}
	if len(record.NotBuilt) != 1 || record.NotBuilt[0].Kind != "test program" || !strings.Contains(record.NotBuilt[0].Reason, "mayMoreStack") {
		t.Fatalf("not built: %+v", record.NotBuilt)
	}
}
