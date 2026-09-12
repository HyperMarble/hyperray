// Memory input conversion carries declared mappings and backing unchanged.
// It must never add an implicit region the caller did not declare.
package main

import (
	"fmt"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func buildMemoryInput(declared machineMemory) (isla.ARM64MemoryInput, error) {
	mappings := make([]isla.MemoryMapping, 0, len(declared.Mappings))
	for _, mapping := range declared.Mappings {
		mappings = append(mappings, isla.MemoryMapping{
			VA: mapping.VA, PA: mapping.PA, Length: mapping.Length,
			Permission: isla.MemoryPermission(mapping.Permission),
		})
	}
	backing, err := memoryBackings(declared.Backing)
	if err != nil {
		return isla.ARM64MemoryInput{}, err
	}
	return isla.NewARM64MemoryInput(isla.ARM64NormalS1Fixed4KSIMD,
		isla.TableArena{Base: declared.TableBase, CapacityPages: declared.TablePages}, mappings, backing)
}

func memoryBackings(declared []machineBacking) ([]isla.MemoryBacking, error) {
	backing := make([]isla.MemoryBacking, 0, len(declared))
	for _, entry := range declared {
		content, err := decodeBackingBytes(entry.Bytes)
		if err != nil {
			return nil, fmt.Errorf("backing at 0x%x: %w", entry.Address, err)
		}
		backing = append(backing, isla.MemoryBacking{
			Address: entry.Address, Permission: isla.MemoryPermission(entry.Permission), Bytes: content,
		})
	}
	return backing, nil
}

func registerValues(declared []machineRegisterPair) []isla.RegisterValue {
	values := make([]isla.RegisterValue, 0, len(declared))
	for _, pair := range declared {
		values = append(values, isla.RegisterValue{Name: pair.Name, Value: pair.Value})
	}
	return values
}

func memoryObservations(declared []machineObservation) []isla.MemoryObservation {
	observations := make([]isla.MemoryObservation, 0, len(declared))
	for _, entry := range declared {
		observations = append(observations, isla.MemoryObservation{
			Name: entry.Name, Address: entry.Address, Bytes: entry.Bytes,
		})
	}
	return observations
}
