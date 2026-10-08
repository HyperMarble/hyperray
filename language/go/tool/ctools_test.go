// Purpose: every variable go help environment describes as a command or a
// tool path is one the record describes; a sixth tool in a future Go
// fails this test by name.
// Never:   trusts a hand-written list of tools over go help environment.
package tool

import (
	"os/exec"
	"regexp"
	"testing"
)

var toolVariable = regexp.MustCompile(`(?m)^\t([A-Z_]+)\n\t\t(?:The command to use|Path to )`)

var describedTools = map[string]bool{"CC": true, "CXX": true, "FC": true, "AR": true, "PKG_CONFIG": true}

func TestEveryToolGoNamesIsDescribed(t *testing.T) {
	help, err := exec.Command("go", "help", "environment").Output()
	if err != nil {
		t.Fatal(err)
	}
	names := toolVariable.FindAllStringSubmatch(string(help), -1)
	if len(names) == 0 {
		t.Fatal("go help environment names no tools")
	}
	for _, name := range names {
		if !describedTools[name[1]] {
			t.Errorf("go names the tool %s and the record does not describe it", name[1])
		}
	}
}
