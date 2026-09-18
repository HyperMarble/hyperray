//go:build isla_integration && arm64_acceptance

// These tests require the measured ARM64 normal-memory native capability.
// They must reject empty reachability, incomplete terminal rows, and weak witnesses.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestRealARM64PacketProcessorAll11CorrectProperties(t *testing.T) {
	content, start, end := packetFixture(t)
	cases := readPacketCases(t, packetFixtureRoot(t))
	assertPacketCaseDomain(t, cases)
	verifier := arm64NormalExecutableVerifier(t)
	for _, testCase := range cases {
		program := buildPacketMemoryProgram(t, content, start, end, testCase, packetExpectedValue(t, testCase))
		assertPacketInitialBinding(t, program, testCase)
		result, err := verifyPacketProgram(t, verifier, program)
		if err != nil {
			t.Fatalf("case %q verification error = %v", testCase.Name, err)
		}
		if result.Verification.Status != isla.Proved {
			t.Fatalf("case %q status = %s, want %s", testCase.Name, result.Verification.Status, isla.Proved)
		}
		if result.Verification.CandidateCount == 0 || len(result.Verification.TerminalEvidence) == 0 || len(result.Execution.Observed) == 0 || len(result.Execution.Threads) == 0 {
			t.Fatalf("case %q lacks nonempty reachability or terminal evidence: %#v", testCase.Name, result)
		}
		if uint64(len(result.Verification.TerminalEvidence)) != result.Verification.CandidateCount {
			t.Fatalf("case %q terminal rows = %d, candidates = %d", testCase.Name, len(result.Verification.TerminalEvidence), result.Verification.CandidateCount)
		}
		assertPacketResultBinding(t, result, program)
	}
}

func TestRealARM64PacketProcessorValidShortWrongClaimHasWitness(t *testing.T) {
	content, start, end := packetFixture(t)
	cases := readPacketCases(t, packetFixtureRoot(t))
	short := findPacketCase(t, cases, "valid_short_control")
	program := buildPacketMemoryProgram(t, content, start, end, short, packetWrongValue)
	assertPacketInitialBinding(t, program, short)
	result, err := verifyPacketProgram(t, arm64NormalExecutableVerifier(t), program)
	if err != nil {
		t.Fatalf("wrong short verification error = %v", err)
	}
	if result.Verification.Status != isla.Disproved || result.Verification.CounterexampleState == "" {
		t.Fatalf("wrong short claim result = %#v", result.Verification)
	}
	assertPacketResultBinding(t, result, program)
	assignments, err := arm64Assignments(result.Verification.CounterexampleState)
	if err != nil {
		t.Fatalf("parse wrong short witness: %v", err)
	}
	assertPacketWitness(t, assignments)
}

func findPacketCase(t *testing.T, cases []packetCase, name string) packetCase {
	t.Helper()
	for _, testCase := range cases {
		if testCase.Name == name {
			return testCase
		}
	}
	t.Fatalf("packet case %q is missing", name)
	return packetCase{}
}
