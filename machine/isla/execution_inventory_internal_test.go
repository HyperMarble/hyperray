// Execution tests retain the static denominator for partial query inventories.
// Missing entry, duplicate, changed, and incomplete reports remain errors.
package isla

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestProgramSemanticsRetainsUnobservedInstructions(t *testing.T) {
	program := internalProgram()
	program.instructions = append(program.instructions, machine.Instruction{Address: 0x1004, Bytes: []byte{0x13, 0, 0, 0}})
	report := entrySemanticReport()
	if err := matchProgramSemantics(program, report); err != nil {
		t.Fatal(err)
	}
	inventory := programExecutionInventory(program, report)
	if len(inventory.Observed) != 1 || len(inventory.NotObserved) != 1 {
		t.Fatalf("partition = %#v", inventory)
	}
	if inventory.Observed[0].Address != 0x1000 || inventory.NotObserved[0].Address != 0x1004 {
		t.Errorf("partition changed addresses: %#v", inventory)
	}
}

func TestProgramSemanticsRejectsInvalidExecutionInventories(t *testing.T) {
	report := entrySemanticReport()
	cases := []SemanticReport{
		{Complete: false, ThreadCount: 1, Instructions: report.Instructions, Threads: report.Threads},
		{Complete: true, ThreadCount: 2, Instructions: report.Instructions, Threads: report.Threads},
		{Complete: true, ThreadCount: 1, Instructions: append(report.Instructions, report.Instructions...), Threads: report.Threads},
		{Complete: true, ThreadCount: 1, Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "ffffffff"}}, Threads: []SemanticThread{{ID: 0, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000}, Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "ffffffff"}}}}},
	}
	for index, invalid := range cases {
		if err := matchProgramSemantics(internalProgram(), invalid); err == nil {
			t.Errorf("accepted invalid inventory %d", index)
		}
	}
}

func TestExecutableJoinRejectsMissingStaticFootprints(t *testing.T) {
	result, err := joinedExecutableResult(internalProgram(), FootprintReport{}, VerificationResult{})
	if err == nil || result.Verification.Status != "" || result.StaticCoverage.Complete {
		t.Errorf("accepted missing static evidence: %#v, %v", result, err)
	}
}

func entrySemanticReport() SemanticReport {
	return SemanticReport{Complete: true, ThreadCount: 1, Instructions: []SemanticInstruction{
		{Address: 0x1000, Encoding: "00000013"},
	}, Threads: []SemanticThread{{ID: 0, EntryAddress: 0x1000, EntryAddresses: []uint64{0x1000}, Instructions: []SemanticInstruction{{Address: 0x1000, Encoding: "00000013"}}}}}
}
