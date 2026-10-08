// Purpose: reject build flags that can change the sources behind a record.
// Never:   reject a safe, explicit read-only module choice.
package goadapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

func TestFlagsCannotReplacePinFiles(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	before, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-mod=mod", "-modfile=other.mod", "-overlay=other.json"} {
		outcome := goadapter.BuildRecordOf(root, out, goadapter.Choice{Flags: []string{flag}})
		if outcome.Status != "blocked" || !strings.Contains(outcome.Reason, flag) {
			t.Fatalf("%s: %+v", flag, outcome)
		}
	}
	after, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("go.mod changed: %v", err)
	}
}

func TestGOFLAGSCannotEnableModuleUpdates(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	t.Setenv("GOFLAGS", "-mod=mod")
	outcome := goadapter.BuildRecordOf(root, out, goadapter.Choice{})
	if outcome.Status != "blocked" || !strings.Contains(outcome.Reason, "GOFLAGS") {
		t.Fatalf("%+v", outcome)
	}
}

func TestExplicitReadonlyModeIsAllowed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	if outcome := goadapter.BuildRecordOf(root, out, goadapter.Choice{Flags: []string{"-mod=readonly"}}); outcome.Status != "built" {
		t.Fatalf("%+v", outcome)
	}
}
