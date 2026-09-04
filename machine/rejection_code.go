// This file names each public loader rejection.
// It must not combine different causes under one profile error.
package machine

type RejectionCode string

const (
	MalformedELF                 RejectionCode = "malformed_elf"
	InvalidLoadCapacity          RejectionCode = "invalid_load_capacity"
	UnsupportedELFClass          RejectionCode = "unsupported_elf_class"
	UnsupportedELFData           RejectionCode = "unsupported_elf_data"
	UnsupportedELFOSABI          RejectionCode = "unsupported_elf_osabi"
	UnsupportedELFABIVersion     RejectionCode = "unsupported_elf_abi_version"
	UnsupportedELFType           RejectionCode = "unsupported_elf_type"
	UnsupportedELFMachine        RejectionCode = "unsupported_elf_machine"
	UnsupportedRISCVFlags        RejectionCode = "unsupported_riscv_flags"
	DynamicExecutable            RejectionCode = "dynamic_executable"
	InvalidLoadFlags             RejectionCode = "invalid_load_flags"
	FileSizeExceedsMemorySize    RejectionCode = "file_size_exceeds_memory_size"
	SegmentAddressOverflow       RejectionCode = "segment_address_overflow"
	SegmentOutsideArtifact       RejectionCode = "segment_outside_artifact"
	InvalidSegmentAlignment      RejectionCode = "invalid_segment_alignment"
	WritableExecutableSegment    RejectionCode = "writable_executable_segment"
	LoadedByteCapacityExceeded   RejectionCode = "loaded_byte_capacity_exceeded"
	OverlappingLoadSegments      RejectionCode = "overlapping_load_segments"
	MissingLoadableBytes         RejectionCode = "missing_loadable_bytes"
	MissingExecutableRegion      RejectionCode = "missing_executable_region"
	UnalignedExecutableRegion    RejectionCode = "unaligned_executable_region"
	TruncatedInstructionEncoding RejectionCode = "truncated_instruction_encoding"
	UnsupportedInstructionLength RejectionCode = "unsupported_instruction_length"
	EntryNotInstruction          RejectionCode = "entry_not_instruction"
)
