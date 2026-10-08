// Purpose: the record names every tool in go's tool directory, one hash over
// the standard library source, and the experiments go env reports.
// Never:   records fewer tools than the directory holds.
package goadapter_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

func TestTheWholeGoInstallationIsHashed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	toolchain := built(t, root, out, goadapter.Choice{}).Toolchain
	settings, err := exec.Command("go", "env", "GOTOOLDIR", "GOEXPERIMENT").Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(settings), "\n")+"\n", "\n")
	entries, err := os.ReadDir(lines[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || len(toolchain.Tools) != len(entries) {
		t.Fatalf("%d tools in the directory, %d in the record", len(entries), len(toolchain.Tools))
	}
	if len(toolchain.StdSource) != 64 || toolchain.Experiments != lines[1] {
		t.Fatalf("toolchain: %+v", toolchain)
	}
}
