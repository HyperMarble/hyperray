// Internal executable tests force both sides of every exact inventory comparison.
package isla

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestFootprintQueryRejectsArtifactMismatches(t *testing.T) {
	digest := strings.Repeat("a", 64)
	request := VerificationRequest{query: Request{
		architecture: Artifact{digest: digest}, configuration: Artifact{digest: digest},
	}}
	report := FootprintReport{Evidence: FootprintEvidence{
		ArchitectureDigest: strings.Repeat("b", 64), ConfigurationDigest: digest,
	}}
	if err := matchFootprintQuery(report, request); err == nil {
		t.Error("matchFootprintQuery() accepted a different architecture")
	}
	report.Evidence.ArchitectureDigest = digest
	report.Evidence.ConfigurationDigest = strings.Repeat("b", 64)
	if err := matchFootprintQuery(report, request); err == nil {
		t.Error("matchFootprintQuery() accepted a different configuration")
	}
}

func TestProgramSemanticsRejectsMissingEntryAndExtraInstructions(t *testing.T) {
	program := internalProgram()
	missing := entrySemanticReport()
	if err := matchProgramSemantics(program, missing); err != nil {
		t.Fatalf("accepted semantic baseline rejected: %v", err)
	}
	missing.Threads[0].Instructions = nil
	failure, ok := matchError(matchProgramSemantics(program, missing))
	if !ok || failure.Code != CoverageMismatch || failure.Subject != "executed instruction inventory" || failure.Detail != "thread entry event missing" {
		t.Fatalf("missing instruction error = %#v, want exact thread-entry mismatch", failure)
	}
	extraInstruction := SemanticInstruction{Address: 0x2000, Encoding: "00000013"}
	extra := entrySemanticReport()
	extra.Threads[0].Instructions = append(extra.Threads[0].Instructions, extraInstruction)
	extra.Instructions = append(extra.Instructions, extraInstruction)
	failure, ok = matchError(matchProgramSemantics(program, extra))
	wantDetail := "extra instruction at 0x2000 with encoding 00000013"
	if !ok || failure.Code != CoverageMismatch || failure.Subject != "executed instruction inventory" || failure.Detail != wantDetail {
		t.Fatalf("extra instruction error = %#v, want exact subject/detail %q/%q", failure, "executed instruction inventory", wantDetail)
	}
}

func TestProgramSemanticsAcceptsRootsAwayFromImageEntry(t *testing.T) {
	program := internalProgram()
	program.entryAddress = 0x3000
	program.threadEntries = []ThreadEntry{{EntryAddress: 0x1000}, {EntryAddress: 0x2000}}
	program.instructions = append(program.instructions, machine.Instruction{Address: 0x2000, Bytes: []byte{0x93, 0x00, 0x00, 0x00}})
	program.instructionCount = 2
	threads := []SemanticThread{
		{ID: 0, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000}, Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "00000013"}}},
		{ID: 1, EntryAddress: 0x2000, EntryAddresses: []uint64{0x2000}, Instructions: []SemanticInstruction{{Address: 0x2000, Encoding: "00000093"}}},
	}
	report := SemanticReport{Complete: true, ThreadCount: 2, Instructions: []SemanticInstruction{
		{Address: 0x1000, Encoding: "00000013"}, {Address: 0x2000, Encoding: "00000093"},
	}, Threads: threads}
	if err := matchProgramSemantics(program, report); err != nil {
		t.Fatalf("explicit roots away from image entry rejected: %v", err)
	}
}

func TestProgramSemanticsRejectsDuplicatePerThreadInventory(t *testing.T) {
	program := internalProgram()
	repeated := SemanticInstruction{Address: 0x1000, Encoding: "00000013"}
	report := SemanticReport{Complete: true, ThreadCount: 1, Instructions: []SemanticInstruction{repeated}, Threads: []SemanticThread{
		{ID: 0, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000}, Instructions: []SemanticInstruction{repeated, repeated}},
	}}
	if err := matchProgramSemantics(program, report); err == nil {
		t.Error("accepted duplicate instruction in one thread inventory")
	}
}

func TestProgramSemanticsRejectsThreadRecordCountAndAggregateMismatch(t *testing.T) {
	program := internalTwoThreadProgram()
	threads := internalTwoThreadRecords()
	base := SemanticReport{Complete: true, ThreadCount: 2, Instructions: []SemanticInstruction{
		{Address: 0x1000, Encoding: "00000013"}, {Address: 0x2000, Encoding: "00000093"},
	}, Threads: threads}
	if err := matchProgramSemantics(program, base); err != nil {
		t.Fatalf("valid two-thread baseline rejected: %v", err)
	}
	cases := []struct {
		name, detail string
		report       SemanticReport
	}{
		{"missing record", "per-thread record count differs", SemanticReport{Complete: true, ThreadCount: 2, Instructions: base.Instructions, Threads: threads[:1]}},
		{"extra record", "per-thread record count differs", SemanticReport{Complete: true, ThreadCount: 2, Instructions: base.Instructions, Threads: append(append([]SemanticThread(nil), threads...), SemanticThread{ID: 2, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000}, Instructions: threads[0].Instructions})}},
		{"aggregate mismatch", "aggregate and per-thread sets differ", SemanticReport{Complete: true, ThreadCount: 2, Instructions: base.Instructions[:1], Threads: threads}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure, ok := matchError(matchProgramSemantics(program, testCase.report))
			if !ok || failure.Subject != "executed instruction inventory" || failure.Detail != testCase.detail {
				t.Fatalf("error = %#v, want subject/detail %q/%q", failure, "executed instruction inventory", testCase.detail)
			}
		})
	}
}

func internalTwoThreadProgram() Program {
	program := internalProgram()
	program.threadEntries = []ThreadEntry{{EntryAddress: 0x1000}, {EntryAddress: 0x2000}}
	program.instructions = append(program.instructions, machine.Instruction{Address: 0x2000, Bytes: []byte{0x93, 0, 0, 0}})
	program.instructionCount = 2
	return program
}

func internalTwoThreadRecords() []SemanticThread {
	return []SemanticThread{
		{ID: 0, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000}, Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "00000013"}}},
		{ID: 1, EntryAddress: 0x2000, EntryAddresses: []uint64{0x2000}, Instructions: []SemanticInstruction{{Address: 0x2000, Encoding: "00000093"}}},
	}
}

func TestProgramSemanticsRequiresEachThreadOwnEntry(t *testing.T) {
	program := internalTwoThreadProgram()
	program.entryAddress = 0x1000
	threads := internalTwoThreadRecords()
	valid := SemanticReport{Complete: true, ThreadCount: 2,
		Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "00000013"}, {Address: 0x2000, Encoding: "00000093"}}, Threads: threads}
	if err := matchProgramSemantics(program, valid); err != nil {
		t.Fatalf("valid two-thread entry baseline rejected: %v", err)
	}
	threads[1].Instructions = nil
	report := SemanticReport{Complete: true, ThreadCount: 2,
		Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "00000013"}, {Address: 0x2000, Encoding: "00000093"}}, Threads: threads}
	failure, ok := matchError(matchProgramSemantics(program, report))
	if !ok || failure.Detail != "thread entry event missing" {
		t.Errorf("error = %#v, want missing thread entry event", failure)
	}
}

func matchError(err error) (*Error, bool) {
	failure, ok := err.(*Error)
	return failure, ok
}

func TestProgramSemanticsRejectsBranchWithUnexpectedFirstPC(t *testing.T) {
	program := internalProgram()
	program.threadEntries = []ThreadEntry{{EntryAddress: 0x1000}}
	program.instructions = append(program.instructions, machine.Instruction{Address: 0x2000, Bytes: []byte{0x13, 0, 0, 0}})
	program.instructionCount = 2
	report := SemanticReport{
		Complete: true, ThreadCount: 1,
		Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "00000013"}, {Address: 0x2000, Encoding: "00000013"}},
		Threads: []SemanticThread{{ID: 0, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000, 0x2000}, Instructions: []SemanticInstruction{
			{Address: 0x1000, Encoding: "00000013"}, {Address: 0x2000, Encoding: "00000013"},
		}}},
	}
	if err := matchProgramSemantics(program, report); err == nil {
		t.Error("accepted a thread with a feasible first PC different from its declared entry")
	}
}

func TestSemanticInstructionSortUsesEncodingAtOneAddress(t *testing.T) {
	values := map[SemanticInstruction]struct{}{
		{Address: 4, Encoding: "b"}: {},
		{Address: 4, Encoding: "a"}: {},
	}
	result := sortedSemanticInstructions(values)
	if result[0].Encoding != "a" || result[1].Encoding != "b" {
		t.Errorf("sorted instructions = %v", result)
	}
}
