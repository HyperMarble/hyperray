// External integration tests consume actual compiler and linker artifacts.
// They skip unless the coordinator supplies a completed build directory.
package compilercatalog_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/compilercatalog"
)

func TestPublicReadJoinActualBuildArtifacts(t *testing.T) {
	directory := os.Getenv("HYPERRAY_COMPILER_INVENTORY_DIR")
	if directory == "" {
		t.Skip("HYPERRAY_COMPILER_INVENTORY_DIR is not set")
	}
	manifest := readManifest(t, filepath.Join(directory, "manifest.json"))
	inventory := readInventory(t, manifest.Inventory.Path)
	result := joinActualArtifacts(t, manifest)
	if result.CompilationID != manifest.CompilationID {
		t.Fatalf("compilation ID = %s, want %s", result.CompilationID, manifest.CompilationID)
	}
	expectedOperations := inventoryPositions(inventory)
	if len(result.Operations) != expectedOperations || len(result.Roots) != len(inventory.Instances) {
		t.Fatalf("actual join lost inventory records: operations=%d/%d roots=%d/%d", len(result.Operations), expectedOperations, len(result.Roots), len(inventory.Instances))
	}
	if result.Image.ArtifactSHA256 != manifest.ELF.SHA256 {
		t.Fatalf("image digest = %s, want %s", result.Image.ArtifactSHA256, manifest.ELF.SHA256)
	}
	if hasRootFacts(inventory) && !hasObligation(result.UnresolvedObligations, "root ABI facts remain unresolved") {
		t.Fatal("root facts were admitted without an unresolved obligation")
	}
	t.Logf("compilation_id=%s operations=%d roots=%d loaded_bytes=%d", result.CompilationID, len(result.Operations), len(result.Roots), len(result.Image.LoadedBytes))
}

func TestPublicBuildCLIProducesJoinableArtifacts(t *testing.T) {
	cli := os.Getenv("HYPERRAY_COMPILER_BUILD_CLI")
	request := os.Getenv("HYPERRAY_COMPILER_BUILD_REQUEST")
	if cli == "" || request == "" {
		t.Skip("HYPERRAY_COMPILER_BUILD_CLI and HYPERRAY_COMPILER_BUILD_REQUEST are not set")
	}
	command := exec.Command(cli, "--request", request)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("public build CLI failed: %v\n%s", err, output)
	}
	var built struct {
		Manifest compilercatalog.BuildManifest `json:"manifest"`
	}
	if err := json.Unmarshal(output, &built); err != nil {
		t.Fatalf("decode public build result: %v\n%s", err, output)
	}
	result := joinActualArtifacts(t, built.Manifest)
	if result.CompilationID != built.Manifest.CompilationID {
		t.Fatalf("CLI compilation ID = %s, want %s", result.CompilationID, built.Manifest.CompilationID)
	}
	t.Logf("CLI build joined: compilation_id=%s operations=%d roots=%d", result.CompilationID, len(result.Operations), len(result.Roots))
}

func readManifest(t *testing.T, path string) compilercatalog.BuildManifest {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest compilercatalog.BuildManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func readInventory(t *testing.T, path string) compilercatalog.Inventory {
	t.Helper()
	artifact, err := compilercatalog.Read(readPath(t, path))
	if err != nil {
		t.Fatal(err)
	}
	return artifact.Inventory
}

func inventoryPositions(inventory compilercatalog.Inventory) int {
	count := 0
	for _, instance := range inventory.Instances {
		if instance.Body != nil {
			count += len(instance.Body.Positions)
		}
	}
	return count
}

func hasRootFacts(inventory compilercatalog.Inventory) bool {
	for _, instance := range inventory.Instances {
		if len(instance.RootFacts) != 0 && string(instance.RootFacts) != "null" {
			return true
		}
	}
	return false
}

func hasObligation(obligations []string, expected string) bool {
	for _, obligation := range obligations {
		if obligation == expected {
			return true
		}
	}
	return false
}

func joinActualArtifacts(t *testing.T, manifest compilercatalog.BuildManifest) compilercatalog.Report {
	t.Helper()
	inventoryContent := readPath(t, manifest.Inventory.Path)
	artifact, err := compilercatalog.Read(inventoryContent)
	if err != nil {
		t.Fatalf("public Read rejected actual inventory: %v", err)
	}
	objects := []compilercatalog.ObjectArtifact{{Path: manifest.Object.Path, Content: readPath(t, manifest.Object.Path)}}
	for _, identity := range manifest.ExternArtifacts {
		objects = append(objects, compilercatalog.ObjectArtifact{Path: identity.Path, Content: readPath(t, identity.Path)})
	}
	if manifest.BoundaryArtifact != nil {
		identity := *manifest.BoundaryArtifact
		objects = append(objects, compilercatalog.ObjectArtifact{Path: identity.Path, Content: readPath(t, identity.Path)})
	}
	result, err := compilercatalog.Join(artifact, manifest, objects, readPath(t, manifest.ELF.Path), 1<<30)
	if err != nil {
		t.Fatalf("public Join rejected actual artifacts: %v", err)
	}
	return result
}

func readPath(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(fmt.Errorf("read %s: %w", path, err))
	}
	return content
}
