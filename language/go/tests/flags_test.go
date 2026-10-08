// Purpose: a flag that lets the build change or swap its module files is not
// refused: -mod=mod records the pin files before and after, -modfile
// records the alternate file, and a plain read-only build is unchanged.
// Never:   lets a changed pin file go unrecorded.
package goadapter_test

import (
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const usesUnpinnedDep = "package lib\n\nimport \"example.com/dep\"\n\nfunc L() int { return dep.V() }\n"

func TestModModeRecordsThePinFilesBeforeAndAfter(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": usesUnpinnedDep})
	dep := filepath.Join(filepath.Dir(root), "dep")
	if err := os.Mkdir(dep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(dep, "dep.go"), localDep)
	writeTestFile(t, filepath.Join(root, "go.mod"), sampleModule+"\nreplace example.com/dep => ../dep\n")
	settings := built(t, root, out, goadapter.Choice{Flags: []string{"-mod=mod"}}).Settings
	if !settings.PinsChanged || len(settings.LockFilesAfter) == 0 || settings.LockFilesAfter[0].Sha256 == settings.LockFiles[0].Sha256 {
		t.Fatalf("settings: %+v", settings)
	}
}

func TestAnAlternateModFileIsHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	writeTestFile(t, filepath.Join(root, "other.mod"), sampleModule)
	settings := built(t, root, out, goadapter.Choice{Flags: []string{"-modfile=other.mod"}}).Settings
	if len(settings.ModFile) != 1 || filepath.Base(settings.ModFile[0].Path) != "other.mod" || settings.ModFile[0].Sha256 == "" {
		t.Fatalf("mod file: %+v", settings.ModFile)
	}
}

func TestExplicitReadonlyModeIsAllowed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	settings := built(t, root, out, goadapter.Choice{Flags: []string{"-mod=readonly"}}).Settings
	if settings.PinsChanged || settings.LockFilesAfter != nil {
		t.Fatalf("settings: %+v", settings)
	}
}
