// Purpose: the record pins the go command, its invoked tools, standard
// library source, version and experiments.
// Never:   treats a version string as the hash of compiler input bytes.
package goadapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
	"github.com/HyperMarble/hyperray/language/go/record"
)

func TestTheGoToolIsRecordedAsGoReportsIt(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	toolchain := built(t, root, out, goadapter.Choice{}).Toolchain
	settings, err := exec.Command("go", "env", "GOVERSION", "GOEXPERIMENT").Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(settings), "\n")+"\n", "\n")
	if toolchain.Version != lines[0] || toolchain.Experiments != lines[1] || len(toolchain.Compiler.Sha256) != 64 {
		t.Fatalf("toolchain: %+v, go env says %q", toolchain, lines)
	}
}

func TestTheRecordPinsTheInvokedGoTools(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	toolchain := built(t, root, out, goadapter.Choice{}).Toolchain
	if len(toolchain.StdSource) != 64 {
		t.Fatalf("standard library source hash missing: %+v", toolchain)
	}
	command := exec.Command("go", "env", "GOTOOLDIR")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	toolDir := strings.TrimSpace(string(output))
	for _, name := range []string{"compile", "link"} {
		path := filepath.Join(toolDir, name)
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256(bytes)
		if !hasToolDigest(toolchain.Tools, path, hex.EncodeToString(want[:])) {
			t.Fatalf("%s is not pinned in the toolchain record", path)
		}
	}
}

func hasToolDigest(tools []record.FileDigest, path, digest string) bool {
	for _, tool := range tools {
		if tool.Path == path && tool.Sha256 == digest {
			return true
		}
	}
	return false
}
