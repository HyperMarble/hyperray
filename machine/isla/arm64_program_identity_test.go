// ARM64 Program identity tests use the public builder and evidence surface.
// They must retain the original image digest and explicit ordered thread.
package isla_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramPreservesIdentityAndThread(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	program, err := isla.BuildARM64Program(content, 32768, arm64ProgramBoundary(start, end))
	if err != nil {
		t.Fatalf("BuildARM64Program() error = %v", err)
	}
	wantDigest := sha256.Sum256(content)
	if program.ImageDigest() != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("ImageDigest() = %s, want source digest", program.ImageDigest())
	}
	if program.ProfileName() != arm64.ProfileName || program.ReturnAddress() != arm64ProgramReturnAddress {
		t.Fatalf("profile/return = %q/%#x", program.ProfileName(), program.ReturnAddress())
	}
	// Only the pages the range reaches are loaded, so the count is the page
	// holding the function rather than the whole file.
	if program.InstructionCount() != 2 || program.LoadedByteCount() != 4096 {
		t.Fatalf("inventory = %d instructions and %d bytes", program.InstructionCount(), program.LoadedByteCount())
	}
	threads := program.ThreadEntries()
	if len(threads) != 1 || threads[0].EntryAddress != start || len(threads[0].InitialRegisters) != 0 {
		t.Fatalf("threads = %#v, want one entry without initial-state registers", threads)
	}
	assertARM64ProgramEvidence(t, program.Evidence(), start, end)
}

func assertARM64ProgramEvidence(t *testing.T, evidence isla.ProgramEvidence, start uint64, end uint64) {
	t.Helper()
	if evidence.Profile != arm64.ProfileName || evidence.FunctionStart != start || evidence.FunctionEnd != end {
		t.Fatalf("evidence boundary/profile = %#v", evidence)
	}
	if evidence.ReturnAddress != arm64ProgramReturnAddress || evidence.ThreadCount != 1 || evidence.ThreadEntries == "" {
		t.Fatalf("evidence return/thread = %#v", evidence)
	}
}
