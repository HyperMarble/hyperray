// This file defines the public rejection code for unsupported segment flags.
// It must not define validity for Mach-O files outside the initial profile.
package arm64

const UnsupportedSegmentFlags RejectionCode = "unsupported_segment_flags"
