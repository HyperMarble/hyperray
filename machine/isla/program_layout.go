// Program layout separates the entry thread from other loaded memory regions.
// Every loaded byte appears in the thread or in one generated section.
package isla

import "github.com/HyperMarble/hyperray/machine"

type programSection struct {
	address     uint64
	bytes       []byte
	permissions machine.Permissions
}

type programLayout struct {
	sections []programSection
}

func layoutProgram(image machine.Image) (programLayout, error) {
	_, _, err := entryRegion(image)
	if err != nil {
		return programLayout{}, err
	}
	return programLayout{sections: loadedSections(image.LoadedBytes)}, nil
}

func entryRegion(image machine.Image) (uint64, uint64, error) {
	for index := range image.ExecutableRegions {
		region := image.ExecutableRegions[index]
		end := region.StartAddress + region.ByteLength
		if image.EntryAddress >= region.StartAddress && image.EntryAddress < end {
			return image.EntryAddress, end, nil
		}
	}
	return 0, 0, engineError(CoverageMismatch, "entry address", "has no executable region")
}

func loadedSections(loaded []machine.LoadedByte) []programSection {
	sections := make([]programSection, 0)
	current := programSection{}
	for index := range loaded {
		value := loaded[index]
		if !adjacentByte(current, value) {
			sections = appendSection(sections, current)
			current = programSection{address: value.Address, permissions: value.Permissions}
		}
		current.bytes = append(current.bytes, value.Value)
	}
	return appendSection(sections, current)
}

func adjacentByte(section programSection, value machine.LoadedByte) bool {
	if len(section.bytes) == 0 || section.address > ^uint64(0)-uint64(len(section.bytes)) {
		return false
	}
	return section.address+uint64(len(section.bytes)) == value.Address && section.permissions == value.Permissions
}

func appendSection(sections []programSection, section programSection) []programSection {
	if len(section.bytes) == 0 {
		return sections
	}
	return append(sections, section)
}
