//go:build isla_integration && rust_acceptance

// This test joins one public Rust build to one Isla verification request.
// It never accepts a stale ELF or a substituted program artifact.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealRustCompilerCatalogReachesIslaVerifier(t *testing.T) {
	manifest := runRealCompilerBuild(t)
	artifact := readRealInventory(t, manifest)
	elfBytes := readRealArtifact(t, manifest.ELF.Path)
	report := joinRealBuild(t, artifact, manifest, elfBytes)
	boundary := rustProgramBoundary(t, elfBytes, 3, 28)
	program, err := isla.BuildProgram(elfBytes, 1<<30, boundary)
	if err != nil {
		t.Fatalf("BuildProgram() rejected joined ELF: %v", err)
	}
	if program.ImageDigest() != report.Image.ArtifactSHA256 {
		t.Fatalf("program image digest = %s, want %s", program.ImageDigest(), report.Image.ArtifactSHA256)
	}
	result := verifyRealRustProgram(t, program)
	assertRealRustResult(t, result, report.Image.ArtifactSHA256)
	assertChangedRealELFRejected(t, artifact, manifest, elfBytes)
}
