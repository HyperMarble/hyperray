// Join tests mutate one accepted evidence property at a time.
// They verify admission errors without asserting semantic coverage.
package compilercatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestJoinReReadsContentAfterParsedInventoryMutation(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	artifact.Inventory.CompilationID = "mutated"
	report, err := Join(artifact, build, objects, elfContent, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if report.CompilationID != "build" {
		t.Fatalf("compilation ID = %s", report.CompilationID)
	}
}

func TestJoinKeepsUnresolvedCompilerObligationsVisible(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	report, err := Join(artifact, build, objects, elfContent, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, report.UnresolvedObligations, "selected sysroot identity remains unresolved")
	assertContains(t, report.UnresolvedObligations, "pre-optimization operation preservation remains unresolved")
}

func TestJoinRejectsTargetMutation(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	build.Target = "aarch64-apple-darwin"
	_, err := Join(artifact, build, objects, elfContent, 1<<20)
	assertExactError(t, err, "unsupported target: aarch64-apple-darwin")
}

func TestJoinRejectsObjectSizeMutation(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	build.Object.Size++
	_, err := Join(artifact, build, objects, elfContent, 1<<20)
	assertExactError(t, err, "object artifact size mismatch: expected "+itoa(build.Object.Size)+", got "+itoa(uint64(len(objects[0].Content))))
}

func TestJoinRejectsELFSizeMutation(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	build.ELF.Size++
	_, err := Join(artifact, build, objects, elfContent, 1<<20)
	assertExactError(t, err, "ELF artifact size mismatch: expected "+itoa(build.ELF.Size)+", got "+itoa(uint64(len(elfContent))))
}

func TestJoinRejectsInventorySizeMutation(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	build.Inventory.Size++
	_, err := Join(artifact, build, objects, elfContent, 1<<20)
	assertExactError(t, err, "inventory artifact size mismatch: expected "+itoa(build.Inventory.Size)+", got "+itoa(uint64(len(artifact.Content))))
}

func TestJoinRejectsMissingReferencedExtern(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	build.ExternArtifacts = []ArtifactIdentity{{Path: "dependency.rlib", SHA256: digestBytes([]byte("dependency")), Size: 10}}
	_, err := Join(artifact, build, objects, elfContent, 1<<20)
	assertExactError(t, err, "missing referenced artifact: dependency.rlib")
}

func TestJoinRejectsExtraObjectArtifact(t *testing.T) {
	artifact, build, objects, elfContent := joinFixture(t)
	objects = append(objects, ObjectArtifact{Path: "extra.o", Content: []byte("extra")})
	_, err := Join(artifact, build, objects, elfContent, 1<<20)
	assertExactError(t, err, "extra object artifact: extra.o")
}

func joinFixture(t *testing.T) (Artifact, BuildManifest, []ObjectArtifact, []byte) {
	t.Helper()
	content, err := json.Marshal(Inventory{Version: 1, CompilationID: "build", Instances: []Instance{}})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := Read(content)
	if err != nil {
		t.Fatal(err)
	}
	elfContent, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "machine", "rv64-lp64d-static.elf"))
	if err != nil {
		t.Fatal(err)
	}
	identity := func(path string, value []byte) ArtifactIdentity {
		return ArtifactIdentity{Path: path, SHA256: digestBytes(value), Size: uint64(len(value))}
	}
	build := BuildManifest{
		Version: 1, CompilationID: "build", Target: "riscv64gc-unknown-linux-gnu",
		Inventory: identity("inventory.json", content), Object: identity("program.o", elfContent), ELF: identity("program.elf", elfContent),
	}
	return artifact, build, []ObjectArtifact{{Path: "program.o", Content: elfContent}}, elfContent
}

func itoa(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func assertContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("missing obligation %q in %#v", expected, values)
}
