// Purpose: a source file that exists only in the project's overlay is part
// of the build, so the record hashes the bytes the overlay brought in.
// Never:   reads a compiled source file from disk when the overlay replaces
// it or adds it.
package goadapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const overlayOnlyFile = "package lib\n\nfunc Extra() int { return 5 }\n"

func TestAFileThatExistsOnlyInTheOverlayIsHashedFromTheOverlay(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	backing := filepath.Join(filepath.Dir(root), "extra_backing.go")
	writeText(t, backing, overlayOnlyFile)
	overlay := filepath.Join(root, "overlay.json")
	writeText(t, overlay, `{"Replace":{"`+filepath.Join(root, "lib", "extra.go")+`":"`+backing+`"}}`)
	record := built(t, root, out, goadapter.Choice{Flags: []string{"-overlay=" + overlay}})
	sum := sha256.Sum256([]byte(overlayOnlyFile))
	for _, source := range record.Artifacts[0].Sources {
		if filepath.Base(source.Path) == "extra.go" && source.Sha256 == hex.EncodeToString(sum[:]) {
			return
		}
	}
	t.Fatalf("extra.go, with the bytes the overlay brought in, is not in the sources: %+v", record.Artifacts[0].Sources)
}
