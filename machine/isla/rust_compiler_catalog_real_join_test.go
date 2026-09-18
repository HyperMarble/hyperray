//go:build isla_integration && rust_acceptance

// These checks preserve the compiler identity through catalog and Isla stages.
// They never accept a changed ELF as a successful build input.
package isla_test

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/compilercatalog"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func readRealInventory(t *testing.T, manifest compilercatalog.BuildManifest) compilercatalog.Artifact {
	t.Helper()
	artifact, err := compilercatalog.Read(readRealArtifact(t, manifest.Inventory.Path))
	if err != nil {
		t.Fatalf("Read() rejected compiler inventory: %v", err)
	}
	return artifact
}

func joinRealBuild(t *testing.T, artifact compilercatalog.Artifact, manifest compilercatalog.BuildManifest, elfBytes []byte) compilercatalog.Report {
	t.Helper()
	report, err := compilercatalog.Join(artifact, manifest, realBuildObjects(t, manifest), elfBytes, 1<<30)
	if err != nil {
		t.Fatalf("Join() rejected compiler artifacts: %v", err)
	}
	return report
}

func assertRealRustResult(t *testing.T, result isla.ExecutableResult, elfDigest string) {
	t.Helper()
	if result.Verification.Status != isla.Proved {
		t.Fatalf("verification status = %s, want %s", result.Verification.Status, isla.Proved)
	}
	if result.Program.ImageDigest != elfDigest {
		t.Fatalf("verification image digest = %s, want %s", result.Program.ImageDigest, elfDigest)
	}
	coverage := result.StaticCoverage
	if !coverage.Complete || coverage.CoveredInstructions == 0 || coverage.CoveredInstructions != coverage.TotalInstructions {
		t.Fatalf("static instruction evidence = %#v", coverage)
	}
}

func assertChangedRealELFRejected(t *testing.T, artifact compilercatalog.Artifact, manifest compilercatalog.BuildManifest, elfBytes []byte) {
	t.Helper()
	mutated := append([]byte(nil), elfBytes...)
	mutated[len(mutated)-1] ^= 1
	_, err := compilercatalog.Join(artifact, manifest, realBuildObjects(t, manifest), mutated, 1<<30)
	if err == nil {
		t.Fatal("Join() accepted an ELF with a changed digest")
	}
	want := "stale ELF artifact: expected " + manifest.ELF.SHA256 + ", got "
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("changed ELF error = %q, want prefix %q", err, want)
	}
}
