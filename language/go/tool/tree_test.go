// Purpose: the tree hash changes when one byte changes or a file is renamed,
// and is the same for the same tree; the folder digest lists every file.
// Never:   gives two different trees the same hash by construction.
package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func hashOf(t *testing.T, files map[string]string) string {
	t.Helper()
	sum, err := treeHash(tree(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestTheTreeHashFollowsContentAndNames(t *testing.T) {
	base := map[string]string{"a/x.go": "package a\n", "b/y.go": "package b\n"}
	same := hashOf(t, base)
	if hashOf(t, base) != same {
		t.Fatal("the same tree hashed differently")
	}
	if hashOf(t, map[string]string{"a/x.go": "package a\n", "b/y.go": "package c\n"}) == same {
		t.Fatal("one changed byte did not change the hash")
	}
	if hashOf(t, map[string]string{"a/x.go": "package a\n", "b/z.go": "package b\n"}) == same {
		t.Fatal("a renamed file did not change the hash")
	}
}

func TestTheFolderDigestListsEveryFile(t *testing.T) {
	root := tree(t, map[string]string{"compile": "1", "link": "2", "sub/skip": "3"})
	found, err := digestFolder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || filepath.Base(found[0].Path) != "compile" || filepath.Base(found[1].Path) != "link" {
		t.Fatalf("digests: %+v", found)
	}
}
