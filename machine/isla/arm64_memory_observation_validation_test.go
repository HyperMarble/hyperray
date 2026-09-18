// ARM64 observation validation tests cover rejected public metadata.
// They must return explicit errors and never create a partial Program.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestBuildARM64ProgramRejectsInvalidMemoryObservations(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	cases := invalidMemoryObservationCases(start)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			boundary := arm64ProgramBoundary(start, end)
			boundary.MemoryObservations = testCase.item
			program, err := isla.BuildARM64Program(content, 32768, boundary)
			if err == nil || program.Content() != nil {
				t.Fatalf("BuildARM64Program() = %#v, want explicit rejection", err)
			}
		})
	}
}

func invalidMemoryObservationCases(start uint64) []struct {
	name string
	item []isla.MemoryObservation
} {
	return []struct {
		name string
		item []isla.MemoryObservation
	}{
		{"empty name", []isla.MemoryObservation{{Address: start, Bytes: 1}}},
		{"punctuation", []isla.MemoryObservation{{Name: "lane-0", Address: start, Bytes: 1}}},
		{"duplicate", []isla.MemoryObservation{{Name: "lane0", Address: start, Bytes: 1}, {Name: "lane0", Address: start + 1, Bytes: 1}}},
		{"register collision", []isla.MemoryObservation{{Name: "R0", Address: start, Bytes: 1}}},
		{"register alias collision", []isla.MemoryObservation{{Name: "X0", Address: start, Bytes: 1}}},
		{"default register collision", []isla.MemoryObservation{{Name: "VBAR_EL1", Address: start, Bytes: 1}}},
		{"default register collision 2", []isla.MemoryObservation{{Name: "CPACR_EL1", Address: start, Bytes: 1}}},
		{"reset register collision", []isla.MemoryObservation{{Name: "InGuardedPage", Address: start, Bytes: 1}}},
		{"reserved collision", []isla.MemoryObservation{{Name: "name", Address: start, Bytes: 1}}},
		{"symbolic collision", []isla.MemoryObservation{{Name: "page_table_base", Address: start, Bytes: 1}}},
		{"type collision", []isla.MemoryObservation{{Name: "uint64_t", Address: start, Bytes: 1}}},
		{"width", []isla.MemoryObservation{{Name: "lane0", Address: start, Bytes: 3}}},
		{"overflow", []isla.MemoryObservation{{Name: "lane0", Address: ^uint64(0), Bytes: 1}}},
		{"image gap", []isla.MemoryObservation{{Name: "lane0", Address: arm64ProgramReturnAddress + 1, Bytes: 1}}},
		{"image endpoint", []isla.MemoryObservation{{Name: "lane0", Address: arm64ProgramReturnAddress, Bytes: 1}}},
	}
}
