// This file validates facts that depend on the framed instruction inventory.
// It must not classify an opcode or claim a semantic case.
package machine

import "fmt"

func validateInstructionInventory(instructions []Instruction, entry uint64, flags uint32) error {
	if flags&riscvCompressedFlag == 0 && containsCompressedInstruction(instructions) {
		return reject(UnsupportedRISCVFlags, "16-bit encoding without EF_RISCV_RVC", nil)
	}
	if !containsInstruction(instructions, entry) {
		detail := fmt.Sprintf("entry 0x%x", entry)
		return reject(EntryNotInstruction, detail, nil)
	}
	return nil
}

func containsCompressedInstruction(instructions []Instruction) bool {
	for _, instruction := range instructions {
		if len(instruction.Bytes) == 2 {
			return true
		}
	}
	return false
}

func containsInstruction(instructions []Instruction, entry uint64) bool {
	for _, instruction := range instructions {
		if instruction.Address == entry {
			return true
		}
	}
	return false
}
