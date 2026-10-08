// Purpose: the record names the source bytes behind each built file: the
// program's sources, the test program's sources with the test files
// too, and a one-byte change moves exactly one hash.
// Never:   lets two different source trees produce the same sources list.
package goadapter_test

import (
	"os"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const sourcedMain = "package main\n\nfunc value() int { return 1 }\nfunc main() { println(value()) }\n"
const sourcedTest = "package main\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = value() }\n"

func TestEachBuiltFileNamesItsSources(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": sourcedMain, "main_test.go": sourcedTest})
	record := built(t, root, out, goadapter.Choice{})
	names := map[string][]string{}
	for _, artifact := range record.Artifacts {
		for _, source := range artifact.Sources {
			names[artifact.Kind] = append(names[artifact.Kind], filepath.Base(source.Path))
		}
	}
	if got := names["program"]; len(got) != 1 || got[0] != "main.go" {
		t.Fatalf("program sources: %v", got)
	}
	if got := names["test program"]; len(got) != 2 || got[0] != "main.go" || got[1] != "main_test.go" {
		t.Fatalf("test program sources: %v", got)
	}
}

func TestAOneByteSourceChangeMovesExactlyThatHash(t *testing.T) {
	root, out := writeModule(t, map[string]string{"main.go": sourcedMain, "main_test.go": sourcedTest})
	before := built(t, root, out, goadapter.Choice{}).Artifacts[1].Sources
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(sourcedMain+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := built(t, root, out, goadapter.Choice{}).Artifacts[1].Sources
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("sources: %v %v", before, after)
	}
	if before[0].Sha256 == after[0].Sha256 || before[1].Sha256 != after[1].Sha256 {
		t.Fatalf("hashes moved wrongly:\n%v\n%v", before, after)
	}
}
