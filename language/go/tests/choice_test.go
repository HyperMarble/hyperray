// Purpose: a build tag switches which file is compiled, and the built file
// says so; an extra flag the user asks for is recorded by Go too.
// Never:   labels a built file with a tag or flag it was not built with.
package goadapter_test

import (
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const plainFile = "//go:build !fancy\n\npackage lib\n\nfunc Total(a, b int) int { return a + b }\n"

const fancyFile = "//go:build fancy\n\npackage lib\n\nfunc Total(a, b int) int { return a*10 + b }\n\nfunc Fancy() int { return 1 }\n"

func settingValue(info goadapter.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func TestATagSwitchesTheCodeAndIsRecorded(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/plain.go": plainFile, "lib/fancy.go": fancyFile})
	off := built(t, root, out, goadapter.Choice{})
	if hasSymbol(symbols(t, off.Artifacts[0].File.Path), "lib.Fancy") {
		t.Fatal("Fancy was built with the tag off")
	}
	on := built(t, root, out, goadapter.Choice{Tags: []string{"fancy"}, Flags: []string{"-gcflags=-N"}})
	info := on.Artifacts[0].BuildInfo
	if !hasSymbol(symbols(t, on.Artifacts[0].File.Path), "lib.Fancy") {
		t.Fatal("Fancy was not built with the tag on")
	}
	if settingValue(info, "-tags") != "fancy" || settingValue(info, "-gcflags") != "-N" {
		t.Fatalf("settings: %+v", info.Settings)
	}
	if len(on.Settings.Requested.Tags) != 1 || on.Settings.Requested.Tags[0] != "fancy" {
		t.Fatalf("requested: %+v", on.Settings.Requested)
	}
}
