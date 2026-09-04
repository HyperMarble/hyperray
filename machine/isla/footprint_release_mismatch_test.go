// Release mismatch tests alter one measured identity at a time.
package isla_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestFootprintReleaseRejectsIdentityMismatch(t *testing.T) {
	for index := 0; index < 4; index++ {
		t.Run(fmt.Sprintf("field-%d", index), func(t *testing.T) {
			engine, architecture, configuration := releaseInputs(t)
			manifest := matchingManifest(engine, architecture, configuration)
			changeManifestIdentity(&manifest, index)
			artifact := manifestArtifact(t, manifest)
			release, err := isla.NewFootprintRelease(artifact, engine, architecture, configuration)
			assertReleaseError(t, index, release, err)
		})
	}
}

func changeManifestIdentity(manifest *testReleaseManifest, index int) {
	changedDigest := strings.Repeat("0", 64)
	switch index {
	case 0:
		manifest.ToolVersion = "different"
	case 1:
		manifest.ToolDigest = changedDigest
	case 2:
		manifest.ArchitectureDigest = changedDigest
	case 3:
		manifest.ConfigurationDigest = changedDigest
	}
}
