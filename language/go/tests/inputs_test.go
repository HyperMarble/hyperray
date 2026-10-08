// Purpose: a user's overlay is merged with the keep overlay and its backing
// files hashed; a toolexec wrapper is hashed; -C makes the named folder
// the project root.
// Never:   refuses one of these flags, or loses the keep file because of one.
package goadapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const overlaidLib = "package lib\n\nfunc Double(a int) int { return a * 2 }\n\nfunc unused(a int) int { return a * 3 }\n\nfunc Overlaid() int { return 7 }\n"

func TestAUserOverlayIsMergedAndItsFilesHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	alt := filepath.Join(out, "alt.go")
	writeTestFile(t, alt, overlaidLib)
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(root, "lib", "lib.go"): alt}})
	if err != nil {
		t.Fatal(err)
	}
	theirs := filepath.Join(out, "user.json")
	writeTestFile(t, theirs, string(overlay))
	record := built(t, root, out, goadapter.Choice{Flags: []string{"-overlay=" + theirs}})
	names := symbols(t, record.Artifacts[0].File.Path)
	if !hasSymbol(names, "lib.Overlaid") || !hasSymbol(names, "lib.unused") {
		t.Fatal("the overlaid file or the keep file is missing from the test program")
	}
	sum := sha256.Sum256([]byte(overlaidLib))
	if record.Settings.Overlay == nil || len(record.Settings.Overlay.Backing) != 1 || record.Settings.Overlay.Backing[0].Sha256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("overlay: %+v", record.Settings.Overlay)
	}
}

func TestTheToolexecWrapperIsHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	wrapper := filepath.Join(out, "wrap.sh")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := built(t, root, out, goadapter.Choice{Flags: []string{"-toolexec=" + wrapper}}).Settings
	sum := sha256.Sum256([]byte("#!/bin/sh\nexec \"$@\"\n"))
	if settings.ToolExec == nil || settings.ToolExec.File.Sha256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("tool exec: %+v", settings.ToolExec)
	}
}

func TestDashCMakesTheNamedFolderTheRoot(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	record := built(t, filepath.Dir(root), out, goadapter.Choice{Flags: []string{"-C", "module"}})
	if len(record.Artifacts) != 1 || len(record.Settings.Requested.Flags) != 2 {
		t.Fatalf("record: %+v", record.Settings)
	}
	for _, flag := range record.Settings.BuildFlags {
		if flag == "-C" || flag == "module" {
			t.Fatalf("-C was passed on: %v", record.Settings.BuildFlags)
		}
	}
}
