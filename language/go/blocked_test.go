// Purpose: a folder without go.mod is blocked; a module whose go.mod does not
// pin what it imports is blocked by Go itself, and go.mod stays as
// it was.
// Never:   lets a blocked outcome look like a build.
package goadapter_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const importsUnpinned = "package lib\n\nimport \"github.com/spf13/pflag\"\n\nvar _ = pflag.Bool\n"

func TestAFolderWithoutGoModIsBlocked(t *testing.T) {
	root := t.TempDir()
	outcome := goadapter.BuildRecordOf(root, t.TempDir(), goadapter.Choice{})
	if outcome.Status != "blocked" || !strings.Contains(outcome.Reason, "no go.mod") {
		t.Fatalf("%+v", outcome)
	}
	written, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "artifacts") {
		t.Fatalf("a blocked outcome carries build fields: %s", written)
	}
}

func TestAnUnpinnedImportIsBlockedAndGoModStaysUnchanged(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": importsUnpinned})
	before, err := os.ReadFile(root + "/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	outcome := goadapter.BuildRecordOf(root, out, goadapter.Choice{})
	if outcome.Status != "blocked" || !strings.Contains(outcome.Reason, "-mod=readonly") {
		t.Fatalf("%+v", outcome)
	}
	after, err := os.ReadFile(root + "/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("go.mod was changed by the build")
	}
}
