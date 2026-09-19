// ARM64 observation names cannot shadow a register the model declares.
// Generated aliases are emitted only in their configured, unpadded spelling.
package isla

import (
	"strconv"
	"strings"
)

// nativeRegisters answers whether a name is a register of the model in use.
type nativeRegisters map[string]struct{}

func newNativeRegisters(names []string) nativeRegisters {
	set := make(nativeRegisters, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set
}

// reserved reports whether an observation name would shadow a register.
//
// The model's own names are decisive. The A64 aliases X0..X30 and W0..W30
// are added because the assertion language accepts them though the model
// declares only R0..R30.
func (registers nativeRegisters) reserved(name string) bool {
	if _, declared := registers[name]; declared {
		return true
	}
	for _, prefix := range []string{"R", "X", "W"} {
		if exactARM64Alias(name, prefix, 30) {
			return true
		}
	}
	return false
}

func exactARM64Alias(name string, prefix string, maximum uint64) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	digits := name[len(prefix):]
	index, err := strconv.ParseUint(digits, 10, 8)
	return err == nil && strconv.FormatUint(index, 10) == digits && index <= maximum
}
