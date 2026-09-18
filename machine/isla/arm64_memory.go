// ARM64 memory types carry the exact finite caller RAM contract.
// They must not infer bytes, permissions, aliases, or translation state.
package isla

// MemoryPermission identifies the access allowed by one mapping or backing.
type MemoryPermission string

const (
	MemoryRead        MemoryPermission = "R"
	MemoryReadWrite   MemoryPermission = "RW"
	MemoryReadExecute MemoryPermission = "RX"
)

// TableArena reserves immutable page-table storage outside caller RAM.
type TableArena struct {
	Base          uint64
	CapacityPages uint32
}

// MemoryMapping maps one identity-mapped, page-aligned range.
type MemoryMapping struct {
	VA         uint64
	PA         uint64
	Length     uint64
	Permission MemoryPermission
}

// MemoryBacking supplies initialized caller bytes at one virtual address.
type MemoryBacking struct {
	Address    uint64
	Permission MemoryPermission
	Bytes      []byte
}

// ARM64MemoryInput binds the selected profile, page tables, mappings, and RAM.
type ARM64MemoryInput struct {
	Profile  MemoryProfile
	Table    TableArena
	Mappings []MemoryMapping
	Backing  []MemoryBacking
}

// NewARM64MemoryInput validates and copies one public memory contract.
func NewARM64MemoryInput(profile MemoryProfile, table TableArena, mappings []MemoryMapping, backing []MemoryBacking) (ARM64MemoryInput, error) {
	input := ARM64MemoryInput{
		Profile: profile, Table: table,
		Mappings: append([]MemoryMapping(nil), mappings...),
		Backing:  copyMemoryBackings(backing),
	}
	if err := validateARM64MemoryInput(input); err != nil {
		return ARM64MemoryInput{}, err
	}
	return input, nil
}

func copyMemoryBackings(values []MemoryBacking) []MemoryBacking {
	result := make([]MemoryBacking, len(values))
	for index := range values {
		result[index] = values[index]
		result[index].Bytes = append([]byte(nil), values[index].Bytes...)
	}
	return result
}
