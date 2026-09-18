// ARM64 memory rendering emits the reviewed table and explicit caller bytes.
// It must not add implicit fill, duplicate fields, or executable RAM backing.
package isla

import (
	"encoding/hex"
	"fmt"
)

func writeARM64Memory(output *limitedBuffer, input *ARM64MemoryInput) {
	if input == nil {
		return
	}
	appendProgramText(output, "\n[arm64_memory]\n")
	appendProgramText(output, fmt.Sprintf("table_base = %q\n", formatHex(input.Table.Base)))
	appendProgramText(output, fmt.Sprintf("table_capacity_pages = %d\n", input.Table.CapacityPages))
	appendProgramText(output, "mappings = [")
	for index, mapping := range input.Mappings {
		if index != 0 {
			appendProgramText(output, ", ")
		}
		appendProgramText(output, fmt.Sprintf("{ va = %q, pa = %q, length = %q, permission = %q }", formatHex(mapping.VA), formatHex(mapping.PA), formatHex(mapping.Length), mapping.Permission))
	}
	appendProgramText(output, "]\nbacking = [")
	for index, backing := range input.Backing {
		if index != 0 {
			appendProgramText(output, ", ")
		}
		appendProgramText(output, fmt.Sprintf("{ address = %q, permission = %q, bytes = %q }", formatHex(backing.Address), backing.Permission, hex.EncodeToString(backing.Bytes)))
	}
	appendProgramText(output, "]\n")
}

func formatHex(value uint64) string { return fmt.Sprintf("0x%x", value) }
