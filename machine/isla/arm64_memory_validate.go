// ARM64 memory validation checks the finite public contract before rendering.
// It must reject malformed ranges before any native execution request exists.
package isla

const (
	arm64MemoryPageSize = uint64(0x1000)
	arm64MemoryLimit    = uint64(1) << 48
)

func validateARM64MemoryInput(input ARM64MemoryInput) error {
	if input.Profile != ARM64NormalS1Fixed4KSIMD {
		return engineError(InvalidInput, "ARM64 memory profile", string(input.Profile))
	}
	tableEnd, err := validateTableArena(input.Table)
	if err != nil {
		return err
	}
	if len(input.Mappings) == 0 {
		return engineError(InvalidInput, "ARM64 memory mappings", "no mappings declared")
	}
	if err := validateMappings(input.Mappings, tableEnd, input.Table.Base); err != nil {
		return err
	}
	requiredPages, ok := requiredTableCapacityPages(input.Mappings)
	if !ok || uint64(input.Table.CapacityPages) < requiredPages {
		return engineError(InvalidInput, "ARM64 table arena", "capacity is below the checked four-level mapping budget")
	}
	return validateBackings(input.Backing, input.Mappings, input.Table.Base, tableEnd)
}

func requiredTableCapacityPages(mappings []MemoryMapping) (uint64, bool) {
	var mappedPages uint64
	for _, mapping := range mappings {
		pages := mapping.Length / arm64MemoryPageSize
		if pages > (^uint64(0)-mappedPages-1)/4 {
			return 0, false
		}
		mappedPages += pages
	}
	return 1 + 4*mappedPages, true
}

func validateTableArena(table TableArena) (uint64, error) {
	if table.Base%arm64MemoryPageSize != 0 || table.CapacityPages == 0 {
		return 0, engineError(InvalidInput, "ARM64 table arena", "base is unaligned or capacity is zero")
	}
	length := uint64(table.CapacityPages) * arm64MemoryPageSize
	if length/arm64MemoryPageSize != uint64(table.CapacityPages) {
		return 0, engineError(InvalidInput, "ARM64 table arena", "capacity overflows")
	}
	end, ok := finiteRange(table.Base, length)
	if !ok {
		return 0, engineError(InvalidInput, "ARM64 table arena", "range is outside the ARM64 address domain")
	}
	return end, nil
}

func validateMappings(values []MemoryMapping, tableEnd uint64, tableBase uint64) error {
	for index, mapping := range values {
		end, err := validateMapping(mapping, tableBase, tableEnd)
		if err != nil {
			return mappingError(index, err)
		}
		for previousIndex := 0; previousIndex < index; previousIndex++ {
			if rangesOverlap(mapping.VA, end, values[previousIndex].VA, values[previousIndex].VA+values[previousIndex].Length) || rangesOverlap(mapping.PA, end, values[previousIndex].PA, values[previousIndex].PA+values[previousIndex].Length) {
				return engineError(InvalidInput, "ARM64 memory mappings", "ranges overlap")
			}
		}
	}
	return nil
}

func validateMapping(mapping MemoryMapping, tableBase uint64, tableEnd uint64) (uint64, error) {
	if mapping.VA != mapping.PA || mapping.Length == 0 || mapping.VA%arm64MemoryPageSize != 0 || mapping.PA%arm64MemoryPageSize != 0 || mapping.Length%arm64MemoryPageSize != 0 {
		return 0, engineError(InvalidInput, "ARM64 memory mapping", "mapping is not identity mapped and page aligned")
	}
	end, ok := finiteRange(mapping.VA, mapping.Length)
	if !ok || rangesOverlap(mapping.VA, end, tableBase, tableEnd) || !validMemoryPermission(mapping.Permission) {
		return 0, engineError(InvalidInput, "ARM64 memory mapping", "range, table, or permission is invalid")
	}
	return end, nil
}
