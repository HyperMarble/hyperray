// This file accepts the supported RISC-V ELF machine and flags.
// It must not parse ELF containers or infer instruction-set coverage.
package riscv

import "debug/elf"

const (
	// CompressedFlag identifies the optional RVC encoding in ELF flags.
	CompressedFlag         uint32 = 0x0001
	riscvFloatABIMask             = 0x0006
	riscvFloatABIDouble           = 0x0004
	riscvAcceptedFlagsMask        = CompressedFlag | riscvFloatABIMask
)

// AcceptsMachine reports whether machine is the supported RISC-V ELF machine.
func AcceptsMachine(machine elf.Machine) bool {
	return machine == elf.EM_RISCV
}

// AcceptsFlags reports whether flags select LP64D with optional RVC only.
func AcceptsFlags(flags uint32) bool {
	return flags&riscvFloatABIMask == riscvFloatABIDouble && flags&^riscvAcceptedFlagsMask == 0
}
