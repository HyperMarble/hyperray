// Purpose: use a module's vendor tree when Go's default build would use it.
// Never:   silently switch a vendored project to module-cache source.
package goadapter_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const moduleWithVendor = "module example.com/sample\n\ngo 1.25.0\n\nrequire example.com/dep v0.0.0\nreplace example.com/dep => ../dep\n"
const depSource = "package dep\nfunc Value() int { return 1 }\n"
const usesDep = "package lib\nimport \"example.com/dep\"\nfunc Value() int { return dep.Value() }\n"
const vendorCheck = "package lib\nimport \"testing\"\nfunc TestUsesVendor(t *testing.T) { if Value() != 2 { t.Fatal(\"vendor was ignored\") } }\n"

func TestDefaultBuildUsesVendoredCode(t *testing.T) {
	root, out := writeModule(t, map[string]string{
		"lib/lib.go": usesDep, "lib/lib_test.go": vendorCheck,
	})
	base := filepath.Dir(root)
	dep := filepath.Join(base, "dep")
	if err := os.Mkdir(dep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dep, "go.mod"), "module example.com/dep\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(dep, "dep.go"), depSource)
	writeTestFile(t, filepath.Join(root, "go.mod"), moduleWithVendor)
	command := exec.Command("go", "mod", "vendor")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go mod vendor: %s: %v", output, err)
	}
	writeTestFile(t, filepath.Join(root, "vendor", "example.com", "dep", "dep.go"),
		"package dep\nfunc Value() int { return 2 }\n")
	record := built(t, root, out, goadapter.Choice{})
	command = exec.Command(record.Artifacts[0].File.Path, "-test.run=TestUsesVendor")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("test binary used other dependency source: %s: %v", output, err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
