// This file names observable ARM64 Mach-O loader failures.
// It must not hide unsupported commands or malformed metadata.
package arm64

const (
	InvalidLoadCapacity          RejectionCode = "invalid_load_capacity"
	LoadedByteCapacityExceeded   RejectionCode = "loaded_byte_capacity_exceeded"
	UnsupportedLoadCommand       RejectionCode = "unsupported_load_command"
	InvalidLoadCommand           RejectionCode = "invalid_load_command"
	SegmentOverlap               RejectionCode = "segment_overlap"
	FileBackedOverlap            RejectionCode = "file_backed_overlap"
	MissingTextSegment           RejectionCode = "missing_text_segment"
	InvalidPagezero              RejectionCode = "invalid_pagezero"
	InvalidSection               RejectionCode = "invalid_section"
	UnsupportedSectionType       RejectionCode = "unsupported_section_type"
	UnsupportedSectionAttributes RejectionCode = "unsupported_section_attributes"
	SectionOverlap               RejectionCode = "section_overlap"
	MixedInstructionData         RejectionCode = "mixed_instruction_data"
	InvalidSymbolTable           RejectionCode = "invalid_symbol_table"
	UndefinedSymbol              RejectionCode = "undefined_symbol"
	IndirectSymbol               RejectionCode = "indirect_symbol"
	InvalidSymbolSection         RejectionCode = "invalid_symbol_section"
	UnsupportedSymbolType        RejectionCode = "unsupported_symbol_type"
	InvalidThreadState           RejectionCode = "invalid_thread_state"
	InvalidFunctionBoundary      RejectionCode = "invalid_function_boundary"
	NoPureCode                   RejectionCode = "no_pure_code"
)
