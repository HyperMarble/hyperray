// Package machine loads one static RV64 ELF image profile.
// It never assigns instruction semantics or accepts dynamic code.
package machine

const ProfileName = "static-little-endian-rv64-linux-elf-lp64d"

// Permissions records the access flags of one load segment.
type Permissions struct {
	Readable   bool
	Writable   bool
	Executable bool
}

// LoadedByte records one byte in the memory image.
type LoadedByte struct {
	Address     uint64
	Value       byte
	Permissions Permissions
}

// ExecutableRegion records one half-open executable address range.
type ExecutableRegion struct {
	StartAddress uint64
	ByteLength   uint64
}

// Instruction records one framed encoding without semantic classification.
type Instruction struct {
	Address uint64
	Bytes   []byte
}

// Image is the complete accepted ELF load image.
type Image struct {
	Profile            string
	ArtifactSHA256     string
	EntryAddress       uint64
	ELFFlags           uint32
	MaximumLoadedBytes uint64
	LoadedBytes        []LoadedByte
	ExecutableRegions  []ExecutableRegion
	Instructions       []Instruction
}
