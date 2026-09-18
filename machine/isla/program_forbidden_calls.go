// Model-call requirements add explicit safety conditions to a program.
// They must not depend on source-language patterns or mutate caller input.
package isla

import (
	"sort"
	"strconv"
	"strings"
)

func validateForbiddenCalls(boundary ProgramBoundary) error {
	if len(boundary.ForbiddenModelCalls) != 0 && boundary.MemoryProfile != SequentialMemory {
		return engineError(InvalidInput, "forbidden model calls", "requires sequential memory")
	}
	seen := make(map[string]bool)
	for _, name := range boundary.ForbiddenModelCalls {
		if !identifier(name) || seen[name] {
			return engineError(InvalidInput, "forbidden model calls", "invalid or duplicate name: "+name)
		}
		seen[name] = true
	}
	return nil
}

func writeForbiddenCalls(output *limitedBuffer, names []string) {
	if len(names) == 0 {
		return
	}
	values := append([]string(nil), names...)
	sort.Strings(values)
	for index, name := range values {
		values[index] = strconv.Quote(name)
	}
	appendProgramText(output, "forbidden_model_calls = ["+strings.Join(values, ", ")+"]\n")
}
