// Purpose: every mode go help buildmode names has a label, and the table
// names no mode go dropped.
// Never:   trusts a hand-written mode list over go help buildmode.
package build

import (
	"os/exec"
	"regexp"
	"testing"
)

var buildMode = regexp.MustCompile(`(?m)^\t-buildmode=([a-z-]+)`)

func TestEveryBuildModeHasALabel(t *testing.T) {
	help, err := exec.Command("go", "help", "buildmode").Output()
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, mode := range buildMode.FindAllStringSubmatch(string(help), -1) {
		named[mode[1]] = true
	}
	if len(named) == 0 {
		t.Fatal("go help buildmode names no modes")
	}
	for mode := range named {
		if kindOf[mode] == "" {
			t.Errorf("go has -buildmode=%s and the adapter has no label for it", mode)
		}
	}
	for mode := range kindOf {
		if !named[mode] {
			t.Errorf("the table names %s and go help buildmode does not", mode)
		}
	}
}
