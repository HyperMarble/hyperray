// Purpose: with cgo on, the record names the C++ compiler, the archiver and
// the linker by hash; a prebuilt object file in a package is listed as
// native code.
// Never:   records a cgo build without the tools that made it.
package goadapter_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

func cgoOn(t *testing.T) {
	t.Helper()
	enabled, err := exec.Command("go", "env", "CGO_ENABLED").Output()
	if err != nil || strings.TrimSpace(string(enabled)) != "1" {
		t.Skip("cgo is off")
	}
}

func TestTheToolsBesideTheCCompilerAreHashed(t *testing.T) {
	cgoOn(t)
	root, out := writeModule(t, map[string]string{"lib/cgo.go": cgoSource})
	tools := built(t, root, out, goadapter.Choice{}).CToolchain
	if tools == nil || tools.Cxx == nil || tools.Archiver == nil || tools.Linker == nil {
		t.Fatalf("c toolchain: %+v", tools)
	}
	for _, tool := range []string{tools.Cxx.File.Sha256, tools.Archiver.File.Sha256, tools.Linker.File.Sha256} {
		if len(tool) != 64 {
			t.Fatalf("not a hash: %q", tool)
		}
	}
}

func TestAPrebuiltObjectFileIsListedAsNativeCode(t *testing.T) {
	cgoOn(t)
	root, out := writeModule(t, map[string]string{"lib/lib.go": library})
	source := filepath.Join(root, "lib", "one.c")
	if err := os.WriteFile(source, []byte("int hyperray_one(void) { return 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compile := exec.Command("cc", "-c", source, "-o", filepath.Join(root, "lib", "one.syso"))
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("cc: %s", output)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	record := built(t, root, out, goadapter.Choice{})
	if len(record.NativeCode) != 1 || len(record.NativeCode[0].ObjectFiles) != 1 || record.NativeCode[0].ObjectFiles[0] != "one.syso" {
		t.Fatalf("native code: %+v", record.NativeCode)
	}
}
