// ARM64 program rendering emits native architecture and reset input syntax.
// It retains every loaded byte as data and never invents instruction effects.
package isla

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/HyperMarble/hyperray/machine"
)

func renderARM64Program(image machine.Image, boundary ARM64ProgramBoundary, registers []RegisterValue) ([]byte, error) {
	layout, err := layoutProgram(image)
	if err != nil {
		return nil, err
	}
	output := newLimitedBuffer(boundary.MaximumProgramBytes)
	within := instructionsUnderProof(image.Instructions, boundary.FunctionStart, boundary.FunctionEnd)
	writeARM64Header(output, boundary, registers, within)
	writeARM64Memory(output, boundary.Memory)
	writeARM64Observations(output, boundary.MemoryObservations)
	writeSections(output, layout.sections)
	writeProgramFinal(output, boundary.NegatedAssertion)
	if output.exceeded {
		return nil, engineError(ResourceLimit, "generated ARM64 program", "program size limit reached")
	}
	return []byte(output.String()), nil
}

func writeARM64Header(output *limitedBuffer, boundary ARM64ProgramBoundary, registers []RegisterValue, instructions []machine.Instruction) {
	appendProgramText(output, "arch = \"AArch64\"\n")
	appendProgramText(output, "name = "+strconv.Quote(boundary.Name)+"\n")
	appendProgramText(output, "symbolic = []\n")
	profile := ARM64FetchOnlyV1
	if boundary.Memory != nil {
		profile = boundary.Memory.Profile
	}
	appendProgramText(output, "memory_profile = "+strconv.Quote(string(profile))+"\n")
	appendProgramText(output, "code_ranges = "+formatCodeRanges(instructions)+"\n")
	appendProgramText(output, "\n[thread.0]\nentry = "+formatAddress(boundary.FunctionStart)+"\n")
	appendProgramText(output, "reset = "+formatRegisterTable(registers)+"\n")
	appendProgramText(output, "return_address = "+formatAddress(boundary.ReturnAddress)+"\n")
	appendProgramText(output, "code = \"\"\n")
}

func formatCodeRanges(instructions []machine.Instruction) string {
	ranges := make([]string, len(instructions))
	for index, instruction := range instructions {
		end := instruction.Address + uint64(len(instruction.Bytes))
		ranges[index] = "[" + formatAddress(instruction.Address) + ", " + formatAddress(end) + "]"
	}
	return "[" + strings.Join(ranges, ", ") + "]"
}

func formatAddress(address uint64) string { return strconv.Quote(fmt.Sprintf("0x%x", address)) }

func formatRegisterTable(values []RegisterValue) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Quote(value.Name) + " = " + strconv.Quote(value.Value)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}
