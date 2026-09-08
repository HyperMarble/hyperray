// This file names segment-reader rejections.
// It must not add loader, runtime, or payload-validation semantics.
package arm64

const (
	UnsupportedSegmentCommand  RejectionCode = "unsupported_segment_command"
	InvalidSegmentCommandShape RejectionCode = "invalid_segment_command_shape"
)
