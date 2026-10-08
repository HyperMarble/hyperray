// Purpose: a library-only module is built into a test program in which every
// function, even one nothing calls, has its own machine code; a
// program module gives its executable too; the record carries Go's
// own build facts and the version-pinning files.
// Never:   depends on a function being reachable to find its machine code.
package goadapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const library = "package lib\n\nfunc Double(a int) int { return a * 2 }\n\nfunc unused(a int) int { return a * 3 }\n"

const program = "package main\n\nimport \"example.com/sample/lib\"\n\nfunc main() { println(lib.Double(2)) }\n"

func built(t *testing.T, root, out string, choice goadapter.Choice) *goadapter.BuildRecord {
	t.Helper()
	outcome := goadapter.BuildRecordOf(root, out, choice)
	if outcome.Status != "built" {
		t.Fatal(outcome.Reason)
	}
	return outcome.BuildRecord
}

func TestALibraryOnlyModuleKeepsEveryFunctionInItsTestProgram(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 1 || record.Artifacts[0].Kind != "test program" {
		t.Fatalf("artifacts: %+v", record.Artifacts)
	}
	names := symbols(t, record.Artifacts[0].File.Path)
	for _, want := range []string{"example.com/sample/lib.Double", "example.com/sample/lib.unused"} {
		if !hasSymbol(names, want) {
			t.Fatalf("%s has no machine code of its own", want)
		}
	}
	if _, err := os.Stat(root + "/lib/hyperray_keep_test.go"); err == nil {
		t.Fatal("the generated test file was written into the module")
	}
}

func TestAProgramModuleGivesTheExecutableAndEveryTestProgram(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library, "main.go": program})
	gitCommit(t, root)
	record := built(t, root, out, goadapter.Choice{})
	kinds := []string{}
	for _, artifact := range record.Artifacts {
		kinds = append(kinds, artifact.Package+": "+artifact.Kind)
	}
	want := []string{"example.com/sample: program", "example.com/sample: test program", "example.com/sample/lib: test program"}
	if len(kinds) != 3 || kinds[0] != want[0] || kinds[1] != want[1] || kinds[2] != want[2] {
		t.Fatalf("got %v, want %v", kinds, want)
	}
	info := record.Artifacts[0].BuildInfo
	if info.GoVersion != record.Toolchain.Version || info.Main.Path != "example.com/sample" {
		t.Fatalf("build info: %+v", info)
	}
	if !hasSetting(info, "vcs.revision") || !hasSetting(info, "GOARCH") {
		t.Fatalf("settings: %+v", info.Settings)
	}
}

func TestTheLockFilesAreRecordedByContent(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	record := built(t, root, out, goadapter.Choice{})
	sum := sha256.Sum256([]byte(sampleModule))
	if len(record.Settings.LockFiles) != 1 || record.Settings.LockFiles[0].Sha256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("lock files: %+v", record.Settings.LockFiles)
	}
	if record.Language != "go" || record.OsBuild == "" || record.Toolchain.Compiler.Sha256 == "" {
		t.Fatalf("record: %+v", record)
	}
}

func hasSetting(info goadapter.BuildInfo, key string) bool {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return true
		}
	}
	return false
}
