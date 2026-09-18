// ARM64 backing validation checks explicit caller bytes and permissions.
// It must reject empty, overlapping, executable, and uncovered RAM.
package isla

import "fmt"

func validateBackings(values []MemoryBacking, mappings []MemoryMapping, tableBase uint64, tableEnd uint64) error {
	for index, backing := range values {
		end, ok := finiteRange(backing.Address, uint64(len(backing.Bytes)))
		if !validBackingRange(backing, end, ok, tableBase, tableEnd) || !coveredByMapping(backing, mappings, end) {
			return fmt.Errorf("backing %d: %w", index, engineError(InvalidInput, "ARM64 memory backing", "range, permission, table, or mapping is invalid"))
		}
		if overlapsEarlierBacking(values, index, backing, end) {
			return engineError(InvalidInput, "ARM64 memory backing", "backing ranges overlap")
		}
	}
	return nil
}

func validBackingRange(backing MemoryBacking, end uint64, finite bool, tableBase uint64, tableEnd uint64) bool {
	return finite && len(backing.Bytes) != 0 && validMemoryPermission(backing.Permission) && !rangesOverlap(backing.Address, end, tableBase, tableEnd)
}

func overlapsEarlierBacking(values []MemoryBacking, index int, backing MemoryBacking, end uint64) bool {
	for previousIndex := 0; previousIndex < index; previousIndex++ {
		previousEnd := values[previousIndex].Address + uint64(len(values[previousIndex].Bytes))
		if rangesOverlap(backing.Address, end, values[previousIndex].Address, previousEnd) {
			return true
		}
	}
	return false
}

func coveredByMapping(backing MemoryBacking, mappings []MemoryMapping, backingEnd uint64) bool {
	for _, mapping := range mappings {
		mappingEnd := mapping.VA + mapping.Length
		if backing.Address >= mapping.VA && backingEnd <= mappingEnd && permissionAllows(mapping.Permission, backing.Permission) {
			return true
		}
	}
	return false
}

func validMemoryPermission(permission MemoryPermission) bool {
	return permission == MemoryRead || permission == MemoryReadWrite || permission == MemoryReadExecute
}

func permissionAllows(mapping MemoryPermission, backing MemoryPermission) bool {
	return mapping == backing && (backing == MemoryRead || backing == MemoryReadWrite)
}

func mappingError(index int, err error) error {
	return fmt.Errorf("mapping %d: %w", index, err)
}
