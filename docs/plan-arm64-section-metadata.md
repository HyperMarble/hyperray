# ARM64 section metadata connection

Status: implementation contract, 2026-09-09. This is not execution support.

## Purpose

The mapper must distinguish instruction sections from headers and data inside
executable segments. The current ReadSegmentHeaders retains only segments.
This patch retains section metadata and its owning command before mapping.

## Source evidence

The installed SDK mach-o/loader.h, struct section_64 at lines 473-488,
defines an 80-byte record with two names, address, size, offset, alignment,
relocation metadata, flags, and three reserved words.
Go debug/macho/file.go:93-103 omits the three reserved words from SectionHeader.
Existing segment_reader_shape.go validates exactly 72 + 80*Nsect bytes.
Existing ReadSegmentHeaders validates the full command table and segment ranges.

## Public contract

Add ReadSectionHeaders(content []byte) ([]SectionRecord, error) to machine/arm64.
SectionRecord contains public CommandIndex uint32, SectionIndex uint32,
Segment macho.SegmentHeader, Header macho.SectionHeader, and Reserved1,
Reserved2, Reserved3 uint32 fields. SectionIndex is local to its segment.
CommandIndex counts all load commands, including uninterpreted commands.

Call existing ReadSegmentHeaders first. Preserve its exact first error and
return nil records on error. Then decode only the already bounded section
records, in command order and section order. Retain all source-defined fields.
Names terminate at their first zero byte or use all 16 bytes. Returned names
and records must remain independent of later input mutations.
Do not allocate from Nsect until existing shape validation succeeds.
No segment, section, relocation, or code bytes are materialized by this API.

This API does not validate section mapping, alignment, relocation semantics,
section types, permissions, overlap, code inventory, or runtime obligations.
It must preserve unsupported metadata rather than claim it is accepted code.
An empty structurally valid artifact returns empty metadata, not loadability.
No public generic reader, registry, or parser framework is necessary.

## Acceptance

External arm64_test callers use the public API and actual fixture bytes.
Require at least one fixture section and one file-backed code section.
Compare all standard fields with debug/macho on this trusted fixture only.
The comparison parser is not a production preflight substitute.

Synthetic records exercise every field, all three reserved words, full-width
names, embedded zeros, multiple sections and segments, and an intervening
unknown command. Assert literal expected values and exact owner indices.
Mutate input after return and require unchanged metadata.
Malformed command/segment cases must return nil results with the same typed
code and detail as ReadSegmentHeaders. Include malformed later segments after
valid earlier sections, huge Nsect, and truncated records.
Empty valid input has no section records. No tests execute subject code.

Use red-first tests, then implementation. Run gofmt, go test -count=1
./machine/... ./coverage/compilercatalog, and go vet for the same packages.
Independent review precedes a logical commit. Push remains on hold until the
requested ARM64 end-to-end acceptance succeeds. Preserve staged flags and all
unrelated work. This patch does not modify the public Isla Program API.
