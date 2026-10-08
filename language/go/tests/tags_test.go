// Purpose: the record's tags are the ones Go really used, whichever -tags
// came last, and under -trimpath the ldflags Go drops from its stamp still
// appear in the record's build flags.
// Never:   names a tag the build did not use, or loses a flag the build ran with.
package goadapter_test

import (
	"slices"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const taggedFile = "//go:build viaenv\n\npackage lib\n\nfunc Tagged() int { return 1 }\n"

func TestTagsFromGoflagsAreRecordedAsUsed(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=viaenv")
	root, out := writeModule(t, map[string]string{"lib/lib.go": library, "lib/tagged.go": taggedFile})
	settings := built(t, root, out, goadapter.Choice{}).Settings
	if !slices.Equal(settings.Tags, []string{"viaenv"}) || len(settings.Requested.Tags) != 0 {
		t.Fatalf("settings: %+v", settings)
	}
	if !slices.Contains(settings.BuildFlags, "-tags=viaenv") {
		t.Fatalf("build flags: %v", settings.BuildFlags)
	}
}

func TestTheRequestedTagsReplaceGoflagsTags(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=viaenv")
	root, out := writeModule(t, map[string]string{"lib/lib.go": library, "lib/tagged.go": taggedFile})
	settings := built(t, root, out, goadapter.Choice{Tags: []string{"fancy"}}).Settings
	if !slices.Equal(settings.Tags, []string{"fancy"}) {
		t.Fatalf("tags: %v", settings.Tags)
	}
}

func TestTrimpathKeepsTheLdflagsInTheRecord(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": sourcedMain})
	flags := []string{"-trimpath", "-ldflags=-X main.version=1"}
	record := built(t, root, out, goadapter.Choice{Flags: flags})
	for _, setting := range record.Artifacts[0].BuildInfo.Settings {
		if setting.Key == "-ldflags" {
			t.Fatalf("go stamped ldflags under -trimpath: %+v", setting)
		}
	}
	for _, flag := range flags {
		if !slices.Contains(record.Settings.BuildFlags, flag) {
			t.Fatalf("%s missing from build flags %v", flag, record.Settings.BuildFlags)
		}
	}
}
