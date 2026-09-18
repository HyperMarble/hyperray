//go:build isla_integration && arm64_acceptance

// This external test pins the packet fixture's independent finite-domain specification.
// It must not execute the Mach-O or infer expected results from its disassembly.
package isla_test

import (
	"crypto/sha256"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type packetFixtureManifest struct {
	SourceSHA256      string `json:"source_sha256"`
	ObjectSHA256      string `json:"object_sha256"`
	MachOSHA256       string `json:"macho_sha256"`
	DisassemblySHA256 string `json:"disassembly_sha256"`
	FunctionSymbol    string `json:"function_symbol"`
	FunctionStart     string `json:"function_start"`
	FunctionEnd       string `json:"function_end"`
	TextOffset        uint64 `json:"text_offset"`
	TextSize          uint64 `json:"text_size"`
}

type packetCase struct {
	Name        string `json:"name"`
	InputLength int    `json:"input_length"`
	Bytes       string `json:"bytes"`
	Status      string `json:"status"`
	Value       string `json:"value,omitempty"`
	ErrorCode   uint64 `json:"error_code,omitempty"`
}

func TestARM64PacketFixtureSpecification(t *testing.T) {
	root := packetFixtureRoot(t)
	manifest := readPacketManifest(t, root)
	assertPacketArtifactDigest(t, root, "source.rs", manifest.SourceSHA256)
	assertPacketArtifactDigest(t, root, "source.o", manifest.ObjectSHA256)
	assertPacketArtifactDigest(t, root, "packet-arm64-static", manifest.MachOSHA256)
	assertPacketMachOIdentity(t, root, manifest)
	cases := readPacketCases(t, root)
	assertPacketCaseDomain(t, cases)
	valid, invalid := 0, 0
	for _, testCase := range cases {
		actual := referencePacketResult(t, testCase)
		if actual.status == "ok" {
			valid++
		} else {
			invalid++
		}
		assertPacketCase(t, testCase, actual)
	}
	if valid != 3 || invalid != 8 {
		t.Fatalf("packet case split = %d valid, %d invalid; want 3 and 8", valid, invalid)
	}
}

func assertPacketCaseDomain(t *testing.T, cases []packetCase) {
	t.Helper()
	want := []string{
		"valid_short_control", "valid_medium_data", "valid_long_data",
		"invalid_too_short", "invalid_reserved_flags", "invalid_version",
		"invalid_kind", "invalid_payload_length", "invalid_truncated",
		"invalid_stream", "invalid_checksum",
	}
	if len(cases) != len(want) {
		t.Fatalf("packet cases = %d, want exact eleven finite cases", len(cases))
	}
	seen := make(map[string]bool, len(cases))
	for _, testCase := range cases {
		if seen[testCase.Name] {
			t.Fatalf("packet case %q is duplicated", testCase.Name)
		}
		seen[testCase.Name] = true
	}
	for _, name := range want {
		if !seen[name] {
			t.Fatalf("packet case %q is missing", name)
		}
	}
}

func packetFixtureRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "fixtures", "machine", "arm64", "packet")
}

func readPacketManifest(t *testing.T, root string) packetFixtureManifest {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatalf("read packet manifest: %v", err)
	}
	var manifest packetFixtureManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("decode packet manifest: %v", err)
	}
	for name, digest := range map[string]string{
		"source": manifest.SourceSHA256, "object": manifest.ObjectSHA256,
		"Mach-O": manifest.MachOSHA256, "disassembly": manifest.DisassemblySHA256,
	} {
		if len(digest) != sha256.Size*2 {
			t.Fatalf("%s digest length = %d", name, len(digest))
		}
	}
	if manifest.FunctionSymbol == "" || manifest.FunctionStart == "" || manifest.FunctionEnd == "" || manifest.TextSize == 0 {
		t.Fatal("packet manifest lacks function identity")
	}
	return manifest
}

func assertPacketArtifactDigest(t *testing.T, root string, name string, want string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("read packet %s: %v", name, err)
	}
	digest := sha256.Sum256(content)
	if got := hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("packet %s SHA-256 = %s, want %s", name, got, want)
	}
}

func assertPacketMachOIdentity(t *testing.T, root string, manifest packetFixtureManifest) {
	t.Helper()
	start, err := strconv.ParseUint(manifest.FunctionStart, 0, 64)
	if err != nil {
		t.Fatalf("parse packet function start: %v", err)
	}
	end, err := strconv.ParseUint(manifest.FunctionEnd, 0, 64)
	if err != nil {
		t.Fatalf("parse packet function end: %v", err)
	}
	path := filepath.Join(root, "packet-arm64-static")
	file, err := macho.Open(path)
	if err != nil {
		t.Fatalf("open packet Mach-O: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close packet Mach-O: %v", err)
		}
	}()
	var text *macho.Section
	for _, section := range file.Sections {
		if section.Seg == "__TEXT" && section.Name == "__text" {
			text = section
		}
	}
	if text == nil || start < text.Addr || start >= text.Addr+text.Size || text.Addr+text.Size != end || uint64(text.Offset) != manifest.TextOffset || text.Size != manifest.TextSize {
		t.Fatalf("packet text metadata = %#v, manifest = %#v", text, manifest)
	}
	if file.Symtab == nil {
		t.Fatal("packet Mach-O has no symbol table")
	}
	for _, symbol := range file.Symtab.Syms {
		if symbol.Name == "_arm64_packet_process" {
			if symbol.Value != start {
				t.Fatalf("packet entry address = %#x, want %#x", symbol.Value, start)
			}
			return
		}
	}
	t.Fatal("packet Mach-O lacks _arm64_packet_process")
}

func readPacketCases(t *testing.T, root string) []packetCase {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, "cases.json"))
	if err != nil {
		t.Fatalf("read packet cases: %v", err)
	}
	var cases []packetCase
	if err := json.Unmarshal(content, &cases); err != nil {
		t.Fatalf("decode packet cases: %v", err)
	}
	return cases
}

type packetResult struct {
	status    string
	value     uint64
	errorCode uint64
}

func referencePacketResult(t *testing.T, testCase packetCase) packetResult {
	t.Helper()
	if len(testCase.Bytes) != 64 || testCase.Bytes != strings.ToLower(testCase.Bytes) {
		t.Fatalf("case %q bytes = %q, want 64 lowercase hex characters", testCase.Name, testCase.Bytes)
	}
	buffer, err := hex.DecodeString(testCase.Bytes)
	if err != nil || len(buffer) != 32 {
		t.Fatalf("case %q buffer = %q, want 32 decoded bytes: %v", testCase.Name, testCase.Bytes, err)
	}
	if testCase.InputLength < 0 || testCase.InputLength > len(buffer) {
		t.Fatalf("case %q input length = %d", testCase.Name, testCase.InputLength)
	}
	fail := func(code uint64) packetResult { return packetResult{status: "error", errorCode: code} }
	if testCase.InputLength < 4 {
		return fail(1)
	}
	flags, payloadLength, streamID, expectedChecksum := buffer[0], int(buffer[1]), buffer[2], buffer[3]
	if flags&0xe0 != 0 {
		return fail(2)
	}
	version, kind := flags&0x07, (flags>>3)&0x03
	if version != 2 {
		return fail(3)
	}
	if kind == 0 {
		return fail(4)
	}
	if payloadLength > 28 {
		return fail(5)
	}
	if testCase.InputLength < 4+payloadLength {
		return fail(6)
	}
	if streamID == 0 {
		return fail(7)
	}
	checksum := uint8(0x5a)
	for index := 0; index < payloadLength; index++ {
		checksum = bits.RotateLeft8(checksum, 1) ^ buffer[4+index]
		checksum += uint8(index * 3)
	}
	if checksum != expectedChecksum {
		return fail(8)
	}
	value := uint64(version)<<56 | uint64(kind)<<52 | uint64(streamID)<<32 | uint64(checksum)<<24 | uint64(payloadLength)
	return packetResult{status: "ok", value: value}
}

func assertPacketCase(t *testing.T, testCase packetCase, actual packetResult) {
	t.Helper()
	if testCase.Name == "" {
		t.Fatal("packet case has empty name")
	}
	if testCase.Status != actual.status {
		t.Fatalf("case %q status = %q, want independent result %q", testCase.Name, testCase.Status, actual.status)
	}
	if actual.status == "ok" {
		if testCase.Value == "" {
			t.Fatalf("case %q lacks expected value", testCase.Name)
		}
		var expected uint64
		if _, err := fmt.Sscanf(testCase.Value, "0x%x", &expected); err != nil || expected != actual.value {
			t.Fatalf("case %q value = %q, want 0x%x", testCase.Name, testCase.Value, actual.value)
		}
		return
	}
	if testCase.ErrorCode != actual.errorCode {
		t.Fatalf("case %q error code = %d, want %d", testCase.Name, testCase.ErrorCode, actual.errorCode)
	}
}
