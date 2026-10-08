// Purpose: a default.pgo beside a main package changes its machine code,
// and Go stamps the profile it applied; the record hashes that file.
// Never:   guesses from the folder: the hash comes from the path Go stamped.
package goadapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const profiledMain = "package main\n\nfunc main() { println(1) }\n"

// writeProfile writes a real CPU profile with Go's own profiler.
func writeProfile(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		t.Fatal(err)
	}
	pprof.StopCPUProfile()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestTheProfileGoAppliedIsHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": profiledMain})
	content := writeProfile(t, filepath.Join(root, "default.pgo"))
	record := built(t, root, out, goadapter.Choice{})
	program := record.Artifacts[0]
	if program.Kind != "program" || program.Profile == nil {
		t.Fatalf("program carries no profile: %+v", program)
	}
	sum := sha256.Sum256(content)
	if program.Profile.Sha256 != hex.EncodeToString(sum[:]) || filepath.Base(program.Profile.Path) != "default.pgo" {
		t.Fatalf("profile: %+v", program.Profile)
	}
}

func TestNoProfileMeansNoProfileInTheRecord(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": profiledMain})
	for _, artifact := range built(t, root, out, goadapter.Choice{}).Artifacts {
		if artifact.Profile != nil {
			t.Fatalf("%s carries a profile with none on disk: %+v", artifact.Kind, artifact.Profile)
		}
	}
}
