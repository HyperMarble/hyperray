// Program rendering writes only assembler data directives and quoted query data.
// No source function, instruction value, address, or property is built in.
package isla

import (
	"fmt"
	"strconv"

	"github.com/HyperMarble/hyperray/machine"
)

func renderProgram(image machine.Image, boundary ProgramBoundary) ([]byte, error) {
	threads, err := validateProgramBoundary(image, boundary)
	if err != nil {
		return nil, err
	}
	layout, err := layoutProgram(image)
	if err != nil {
		return nil, err
	}
	output := newLimitedBuffer(boundary.MaximumProgramBytes)
	writeProgramHeader(output, boundary, threads)
	writeSections(output, layout.sections)
	writeProgramFinal(output, boundary.NegatedAssertion)
	if output.exceeded {
		return nil, engineError(ResourceLimit, "generated program", "program size limit reached")
	}
	return []byte(output.String()), nil
}

func writeProgramHeader(output *limitedBuffer, boundary ProgramBoundary, threads []ThreadEntry) {
	appendProgramText(output, "arch = \"RISCV\"\n")
	appendProgramText(output, "name = "+strconv.Quote(boundary.Name)+"\n")
	if boundary.MemoryProfile != AxiomaticMemory {
		appendProgramText(output, "memory_profile = "+strconv.Quote(string(boundary.MemoryProfile))+"\n")
	}
	appendProgramText(output, "symbolic = []\n")
	writeForbiddenCalls(output, boundary.ForbiddenModelCalls)
	writeInitialState(output, boundary.InitialState)
	width := threadNameWidth(len(threads))
	for index := range threads {
		thread := threads[index]
		appendProgramText(output, fmt.Sprintf("\n[thread.%0*d]\nentry = \"0x%x\"\ninit = {", width, index, thread.EntryAddress))
		for registerIndex := range thread.InitialRegisters {
			if registerIndex > 0 {
				appendProgramText(output, ",")
			}
			value := thread.InitialRegisters[registerIndex]
			appendProgramText(output, " "+value.Name+" = "+strconv.Quote(value.Value))
		}
		appendProgramText(output, " }\ncode = \"\"\n")
	}
}

func threadNameWidth(count int) int {
	if count < 10 {
		return 1
	}
	return len(strconv.Itoa(count - 1))
}

func appendProgramText(output *limitedBuffer, value string) {
	output.Write([]byte(value))
}
