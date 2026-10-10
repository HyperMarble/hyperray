// Purpose: the record names the go tool as go itself reports it: version
// and experiments from go env, and the go command by hash.
// Never:   hashes the go installation's own files; go stamps its version
// into every file it builds.
package goadapter_test

import (
	"os/exec"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
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
