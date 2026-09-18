// ARM64 observation rendering emits native metadata with stable field values.
// It must not initialize memory or mutate caller declaration order.
package isla

import (
	"fmt"
	"sort"
)

func writeARM64Observations(output *limitedBuffer, values []MemoryObservation) {
	ordered := copyMemoryObservations(values)
	sort.SliceStable(ordered, func(left int, right int) bool {
		if ordered[left].Address == ordered[right].Address {
			return ordered[left].Name < ordered[right].Name
		}
		return ordered[left].Address < ordered[right].Address
	})
	for _, observation := range ordered {
		appendProgramText(output, fmt.Sprintf("\n[[memory_observations]]\nname = %q\naddress = %q\nbytes = %d\n", observation.Name, formatHex(observation.Address), observation.Bytes))
	}
}
