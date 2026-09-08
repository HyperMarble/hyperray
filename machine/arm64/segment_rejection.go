// This file defines observable ARM64 Mach-O segment range rejections.
// It must not add loader, runtime, or model semantics.
package arm64

const (
	SegmentFileSizeExceedsMemorySize RejectionCode = "segment_file_size_exceeds_memory_size"
	SegmentAddressOverflow           RejectionCode = "segment_address_overflow"
	SegmentOutsideArtifact           RejectionCode = "segment_outside_artifact"
)
