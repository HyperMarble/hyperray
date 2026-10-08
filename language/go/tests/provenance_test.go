// Purpose: preserve package identity, active workspace pins and build inputs.
// Never:   accept a record for a different package or module configuration.
package goadapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

func TestDistinctPackagesHaveDistinctArtifactPaths(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"a/b_c/a.go": "package b_c\nfunc Alpha() int { return 1 }\n",
		"a_b/c/b.go": "package c\nfunc Beta() int { return 2 }\n",
	})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 2 || record.Artifacts[0].File.Path == record.Artifacts[1].File.Path {
		t.Fatalf("artifacts share a path: %+v", record.Artifacts)
	}
	for _, artifact := range record.Artifacts {
		want := map[string]string{
			"example.com/sample/a/b_c": "a/b_c.Alpha",
			"example.com/sample/a_b/c": "a_b/c.Beta",
		}[artifact.Package]
		if want == "" || !hasSymbol(symbols(t, artifact.File.Path), want) {
			t.Fatalf("%s points at the wrong binary", artifact.Package)
		}
	}
}

func TestParentWorkspaceFileIsHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	work := filepath.Join(filepath.Dir(root), "go.work")
	content := []byte("go 1.25.0\n\nuse ./module\n")
	if err := os.WriteFile(work, content, 0o644); err != nil {
		t.Fatal(err)
	}
	record := built(t, root, out, goadapter.Choice{})
	sum := sha256.Sum256(content)
	for _, file := range record.Settings.LockFiles {
		if file.Path == work && file.Sha256 == hex.EncodeToString(sum[:]) {
			return
		}
	}
	t.Fatalf("active workspace missing from lock files: %+v", record.Settings.LockFiles)
}
