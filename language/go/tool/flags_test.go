// Purpose: every flag go help build names has a line in the table saying
// what the adapter does with it, and the table names no flag go dropped.
// Never:   trusts a hand-written flag list over go help build.
package tool

import (
	"os/exec"
	"regexp"
	"testing"
)

var buildFlag = regexp.MustCompile(`(?m)^\t-([a-zA-Z]+)`)

func TestEveryGoBuildFlagHasAHandling(t *testing.T) {
	help, err := exec.Command("go", "help", "build").Output()
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, flag := range buildFlag.FindAllStringSubmatch(string(help), -1) {
		named[flag[1]] = true
	}
	if len(named) == 0 {
		t.Fatal("go help build names no flags")
	}
	for flag := range named {
		if flagHandling[flag] == "" {
			t.Errorf("go build has -%s and the adapter does not say what it does with it", flag)
		}
	}
	for flag := range flagHandling {
		if !named[flag] {
			t.Errorf("the table names -%s and go build does not", flag)
		}
	}
}

func TestFlagValueTakesTheLastOne(t *testing.T) {
	flags := []string{"-tags=a", "-mod", "vendor", "-tags", "b", "-x"}
	if value, found := FlagValue(flags, "tags"); !found || value != "b" {
		t.Fatalf("tags: %q %v", value, found)
	}
	if value, found := FlagValue(flags, "mod"); !found || value != "vendor" {
		t.Fatalf("mod: %q %v", value, found)
	}
	if _, found := FlagValue(flags, "overlay"); found {
		t.Fatal("overlay was not given")
	}
}
