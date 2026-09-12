// Section rendering preserves non-entry bytes at their original addresses.
// The final clause carries only the assertion, which is what Isla reads.
package isla

import (
	"fmt"
	"strconv"
)

func writeSections(output *limitedBuffer, sections []programSection) {
	for index := range sections {
		section := sections[index]
		header := fmt.Sprintf("\n[section.region_%d]\naddress = \"0x%x\"\nwritable = %t\ncode = \"\"\"\n", index, section.address, section.permissions.Writable)
		appendProgramText(output, header)
		writeSectionBytes(output, section.bytes)
		appendProgramText(output, "\"\"\"\n")
	}
}

func writeSectionBytes(output *limitedBuffer, values []byte) {
	for index := range values {
		appendProgramText(output, fmt.Sprintf("    .byte 0x%02x\n", values[index]))
	}
}

func writeProgramFinal(output *limitedBuffer, assertion string) {
	appendProgramText(output, "\n[final]\n")
	appendProgramText(output, "assertion = "+strconv.Quote(assertion)+"\n")
}
