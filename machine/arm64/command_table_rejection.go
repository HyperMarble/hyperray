// This file defines observable ARM64 Mach-O command-table rejections.
// It must not add command semantics, loader behavior, or runtime support.
package arm64

const (
	TruncatedHeader             RejectionCode = "truncated_header"
	CommandTableOutsideArtifact RejectionCode = "command_table_outside_artifact"
	ImpossibleCommandCount      RejectionCode = "impossible_command_count"
	TruncatedCommandPrefix      RejectionCode = "truncated_command_prefix"
	InvalidCommandSize          RejectionCode = "invalid_command_size"
	InvalidCommandAlignment     RejectionCode = "invalid_command_alignment"
	CommandOutsideTable         RejectionCode = "command_outside_table"
	TrailingCommandBytes        RejectionCode = "trailing_command_bytes"
)
