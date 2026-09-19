// ARM64 observation validation accepts only named, finite, backed byte ranges.
// It must not infer bytes from symbolic memory or alter the memory contract.
package isla

import "github.com/HyperMarble/hyperray/machine"

func validateARM64Observations(values []MemoryObservation, image machine.Image, memory *ARM64MemoryInput, registers []RegisterValue, native nativeRegisters) error {
	for index, observation := range values {
		if err := validateARM64ObservationName(values, index, observation.Name, registers, native); err != nil {
			return err
		}
		if !validObservationBytes(observation.Bytes) {
			return engineError(InvalidInput, "ARM64 memory observation width", "must be 1, 2, 4, or 8")
		}
		end, finite := observationRange(observation.Address, observation.Bytes)
		if !finite {
			return engineError(InvalidInput, "ARM64 memory observation range", "address extent is not finite")
		}
		if !observationBacked(observation.Address, end, image, memory) {
			return engineError(InvalidInput, "ARM64 memory observation range", "range is not backed")
		}
	}
	return nil
}

func validateARM64ObservationName(values []MemoryObservation, index int, name string, registers []RegisterValue, native nativeRegisters) error {
	if !plainLine(name) || !identifier(name) {
		return engineError(InvalidInput, "ARM64 memory observation name", name)
	}
	if observationNameReserved(name, registers, native) {
		return engineError(InvalidInput, "ARM64 memory observation name", "collides with native name: "+name)
	}
	for previous := 0; previous < index; previous++ {
		if values[previous].Name == name {
			return engineError(InvalidInput, "ARM64 memory observation name", "duplicate "+name)
		}
	}
	return nil
}

func observationNameReserved(name string, registers []RegisterValue, native nativeRegisters) bool {
	if native.reserved(name) {
		return true
	}
	for _, register := range registers {
		if register.Name == name {
			return true
		}
	}
	return name == "arch" || name == "name" || name == "symbolic" || name == "memory_profile" || name == "code_ranges" || name == "memory_observations" || name == "arm64_memory" || name == "thread" || name == "page_table_base" || name == "s2_page_table_base" || name == "uint8_t" || name == "uint16_t" || name == "uint32_t" || name == "uint64_t"
}

func validObservationBytes(bytes uint32) bool {
	return bytes == 1 || bytes == 2 || bytes == 4 || bytes == 8
}
