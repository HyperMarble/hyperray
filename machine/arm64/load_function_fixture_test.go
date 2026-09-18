// This external test defines the accepted ARM64 image and code inventory.
// It uses the checked-in Mach-O fixture and does not execute it.
package arm64_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestLoadFunctionMapsWholeFixture(t *testing.T) {
	content := fixtureContent(t)
	file := openFixture(t)
	text := arm64Text(t, file)
	image, err := arm64.LoadFunction(content, 32768, arm64.FunctionBoundary{StartAddress: text.Addr, EndAddress: text.Addr + text.Size})
	if err != nil {
		t.Fatalf("LoadFunction() error = %v", err)
	}
	if image.Profile != arm64.ProfileName || image.Profile == machine.ProfileName {
		t.Fatalf("Profile = %q, want distinct ARM64 profile", image.Profile)
	}
	digest := sha256.Sum256(content)
	if image.ArtifactSHA256 != hex.EncodeToString(digest[:]) || image.EntryAddress != text.Addr {
		t.Fatalf("identity = %q, %#x", image.ArtifactSHA256, image.EntryAddress)
	}
	assertFixtureMapping(t, file, content, image)
	assertFixtureInstructions(t, text, content, image)
}
