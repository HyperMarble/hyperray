// Purpose: a folder whose every Go file is for another machine is not a
// refusal: the record has no built file for it and lists the folder under
// excluded with go's own reason and the files by hash; a module where that
// is every folder is a record with zero built files, as go build itself
// treats it.
// Never:   refuses a module go build accepts.
package goadapter_test

import (
	"path/filepath"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const otherMachine = "//go:build amd64 && plan9\n\npackage lib\n\nfunc Elsewhere() int { return 1 }\n"

func TestAModuleWithNothingToBuildIsARecordNotARefusal(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": otherMachine})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 0 || len(record.Excluded) != 1 {
		t.Fatalf("record: %d artifacts, %d excluded", len(record.Artifacts), len(record.Excluded))
	}
	excluded := record.Excluded[0]
	if filepath.Base(excluded.Dir) != "lib" || !strings.Contains(excluded.Reason, "build constraints exclude") {
		t.Fatalf("excluded: %+v", excluded)
	}
	if len(excluded.Files) != 1 || filepath.Base(excluded.Files[0].Path) != "lib.go" || excluded.Files[0].Sha256 == "" {
		t.Fatalf("files: %+v", excluded.Files)
	}
}

func TestAnExcludedFolderBesideABuiltOneIsListed(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library, "other/other.go": otherMachine})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 1 || len(record.Excluded) != 1 || filepath.Base(record.Excluded[0].Dir) != "other" {
		t.Fatalf("record: %d artifacts, excluded %+v", len(record.Artifacts), record.Excluded)
	}
}
