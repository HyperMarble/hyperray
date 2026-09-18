//go:build isla_integration && arm64_acceptance

// This external test sends the real packet Mach-O through the public verifier.
// It must retain unsupported data-memory errors instead of manufacturing success.
package isla_test

import (
	"debug/macho"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

const (
	packetInputAddress  = "0x400000"
	packetInputLength   = "8"
	packetReturn        = uint64(0x100010000)
	packetCorrectValue  = uint64(0x02100021bb000004)
	packetWrongValue    = uint64(0x02100021bc000004)
	packetBaselineError = "isla resource_limit: 0b0400f9: footprint resource limit reached"
)

func TestRealARM64PacketProcessorFetchOnlyBaseline(t *testing.T) {
	content, start, end := packetFixture(t)
	assertPacketReturnOutsideImage(t, packetFixtureRoot(t), packetReturn)
	correct := buildPacketProgram(t, content, start, end, packetCorrectValue)
	wrong := buildPacketProgram(t, content, start, end, packetWrongValue)
	verifier := arm64ExecutableVerifier(t)
	correctResult, correctError := verifyPacketProgram(t, verifier, correct)
	wrongResult, wrongError := verifyPacketProgram(t, verifier, wrong)
	t.Logf("correct claim result=%#v error=%v", correctResult, correctError)
	t.Logf("wrong claim result=%#v error=%v", wrongResult, wrongError)
	assertPacketBaselineError(t, "correct", correctError)
	assertPacketBaselineError(t, "wrong", wrongError)
}

func packetFixture(t *testing.T) ([]byte, uint64, uint64) {
	t.Helper()
	root := packetFixtureRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "packet-arm64-static"))
	if err != nil {
		t.Fatalf("read packet Mach-O: %v", err)
	}
	manifest := readPacketManifest(t, root)
	start, err := strconv.ParseUint(manifest.FunctionStart, 0, 64)
	if err != nil {
		t.Fatalf("parse packet function start: %v", err)
	}
	end, err := strconv.ParseUint(manifest.FunctionEnd, 0, 64)
	if err != nil {
		t.Fatalf("parse packet function end: %v", err)
	}
	return content, start, end
}

func buildPacketProgram(t *testing.T, content []byte, start uint64, end uint64, expected uint64) isla.Program {
	t.Helper()
	boundary := isla.ARM64ProgramBoundary{
		Name:          "arm64-bounded-packet",
		FunctionStart: start,
		FunctionEnd:   end,
		ReturnAddress: packetReturn,
		PostResetRegisters: []isla.RegisterValue{
			{Name: "R0", Value: packetInputAddress},
			{Name: "R1", Value: packetInputLength},
			{Name: "R30", Value: fmt.Sprintf("0x%x", packetReturn)},
			{Name: "SP_EL0", Value: "0x3c40"},
		},
		NegatedAssertion:    packetViolation(expected),
		MaximumProgramBytes: 1 << 20,
	}
	program, err := isla.BuildARM64Program(content, 1<<20, boundary)
	if err != nil {
		t.Fatalf("BuildARM64Program(packet %#x) error = %v", expected, err)
	}
	return program
}

func packetViolation(expected uint64) string {
	return fmt.Sprintf("~((0:R0 = 0x%016x) & (0:SP_EL0 = 0x0000000000003c40) & (0:_PC = 0x%016x))", expected, packetReturn)
}

func verifyPacketProgram(t *testing.T, verifier isla.ExecutableVerifier, program isla.Program) (isla.ExecutableResult, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "packet-program.toml")
	if err := os.WriteFile(path, program.Content(), 0o600); err != nil {
		t.Fatalf("write packet program: %v", err)
	}
	request := arm64VerificationRequest(t, path, program)
	return verifier.VerifyProgram(t.Context(), request, program, isla.ExecutableLimits{
		ThreadLimit: 1, TimeLimitSeconds: 60, MaximumOutputBytes: 32 << 20,
	})
}

func assertPacketBaselineError(t *testing.T, claim string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s claim unexpectedly completed without the measured baseline", claim)
	}
	if err.Error() != packetBaselineError {
		t.Fatalf("%s claim error = %q, want measured baseline %q", claim, err, packetBaselineError)
	}
}

func assertPacketReturnOutsideImage(t *testing.T, root string, address uint64) {
	t.Helper()
	file, err := macho.Open(filepath.Join(root, "packet-arm64-static"))
	if err != nil {
		t.Fatalf("open packet Mach-O for continuation check: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close packet Mach-O for continuation check: %v", err)
		}
	}()
	for _, load := range file.Loads {
		segment, ok := load.(*macho.Segment)
		if ok && address >= segment.Addr && address < segment.Addr+segment.Memsz {
			t.Fatalf("packet continuation %#x is inside loaded segment %s", address, segment.Name)
		}
	}
}
