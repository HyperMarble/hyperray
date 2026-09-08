// This file defines ARM64 Mach-O segment permission rejections.
// It must not add loader, runtime, or protection-transition semantics.
package arm64

const (
	InvalidSegmentProtection        RejectionCode = "invalid_segment_protection"
	SegmentProtectionExceedsMaximum RejectionCode = "segment_protection_exceeds_maximum"
	WritableExecutableSegment       RejectionCode = "writable_executable_segment"
)
