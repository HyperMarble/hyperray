// These tests examine the public identity of an accepted machine image.
// They must bind the result to the exact fixture artifact.
package machine_test

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestLoadRecordsArtifactIdentity(t *testing.T) {
	image, content := acceptedFixture(t, "rv64-lp64d-static.elf")
	digest := sha256.Sum256(content)
	wantDigest := hex.EncodeToString(digest[:])
	if image.Profile != machine.ProfileName {
		t.Errorf("Profile = %q, want %q", image.Profile, machine.ProfileName)
	}
	if image.ArtifactSHA256 != wantDigest {
		t.Errorf("ArtifactSHA256 = %q, want %q", image.ArtifactSHA256, wantDigest)
	}
	if image.MaximumLoadedBytes != uint64(len(content)) {
		t.Errorf("MaximumLoadedBytes = %d, want %d", image.MaximumLoadedBytes, len(content))
	}
	if len(image.Instructions) == 0 {
		t.Fatal("Instructions is empty")
	}
	if image.EntryAddress != image.Instructions[0].Address {
		t.Errorf("EntryAddress = %#x, first instruction = %#x", image.EntryAddress, image.Instructions[0].Address)
	}
}

func TestLoadAcceptsLinuxOSABI(t *testing.T) {
	content := fixture(t, "rv64-lp64d-static.elf")
	content[7] = byte(elf.ELFOSABI_LINUX)
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatalf("machine.Load() error = %v", err)
	}
	flags := binary.LittleEndian.Uint32(content[elfFlagsOffset : elfFlagsOffset+4])
	if image.ELFFlags != flags {
		t.Errorf("ELFFlags = %#x, want %#x", image.ELFFlags, flags)
	}
}

func TestLoadAcceptsOnlyFourByteEncodings(t *testing.T) {
	image, content := acceptedFixture(t, "rv64-lp64d-no-rvc.elf")
	if len(content) == 0 {
		t.Fatal("fixture is empty")
	}
	for _, instruction := range image.Instructions {
		if len(instruction.Bytes) != 4 {
			t.Errorf("instruction at %#x has %d bytes", instruction.Address, len(instruction.Bytes))
		}
	}
}
