// Purpose: every file list `go list` can report is either hashed into the
// record or named here as not compiled; a new kind in a future Go
// fails this test by name.
// Never:   trusts a hand-written list of kinds over `go help list`.
package goadapter

import (
	"os/exec"
	"regexp"
	"testing"
)

var fileListField = regexp.MustCompile(`(?m)^\s+(\w+Files)\s+\[\]string`)

func TestEveryFileListGoListReportsIsHashedOrNamedExcluded(t *testing.T) {
	help, err := exec.Command("go", "help", "list").Output()
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, kind := range programFileKinds {
		known[kind] = true
	}
	for _, kind := range testFileKinds {
		known[kind] = true
	}
	for _, kind := range uncompiledFileKinds {
		known[kind] = true
	}
	fields := fileListField.FindAllStringSubmatch(string(help), -1)
	if len(fields) == 0 {
		t.Fatal("go help list names no file lists")
	}
	for _, field := range fields {
		if !known[field[1]] {
			t.Errorf("go list reports %s and the adapter does not say whether it is compiled", field[1])
		}
	}
	for kind := range (Package{}).fileLists() {
		if !known[kind] {
			t.Errorf("%s is hashed but not named as a kind", kind)
		}
	}
}
