// This file frames pure ARM64 section bytes as four-byte encodings.
// It must not decode opcodes or include headers, padding, or data.
package arm64

import (
	"sort"

	"github.com/HyperMarble/hyperray/machine"
)

func recordCode(content []byte, sections []SectionRecord) ([]machine.ExecutableRegion, []machine.Instruction, error) {
	ordered := append([]SectionRecord(nil), sections...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Header.Addr < ordered[right].Header.Addr })
	regions := make([]machine.ExecutableRegion, 0)
	instructions := make([]machine.Instruction, 0)
	for _, section := range ordered {
		if !isPureInstructionSection(section.Header) {
			continue
		}
		regions = append(regions, machine.ExecutableRegion{StartAddress: section.Header.Addr, ByteLength: section.Header.Size})
		instructions = append(instructions, frameCode(content, section)...)
	}
	regions = mergeCodeRegions(regions)
	if len(regions) == 0 {
		return nil, nil, &Rejection{Code: NoPureCode, Detail: "no pure instruction section"}
	}
	return regions, instructions, nil
}

func frameCode(content []byte, section SectionRecord) []machine.Instruction {
	instructions := make([]machine.Instruction, 0, int(section.Header.Size/4))
	for offset := uint64(0); offset < section.Header.Size; offset += 4 {
		start := uint64(section.Header.Offset) + offset
		bytes := append([]byte(nil), content[int(start):int(start+4)]...)
		instructions = append(instructions, machine.Instruction{Address: section.Header.Addr + offset, Bytes: bytes})
	}
	return instructions
}
