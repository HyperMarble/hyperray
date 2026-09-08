// This file validates the outer 64-bit Mach-O command-table framing.
// It must not parse command payloads, load memory, or imply runtime support.
package arm64

import "fmt"

const (
	machHeader64Size      = 32
	loadCommandHeaderSize = 8
)

// ValidateCommandTable validates a thin ARM64 Mach-O header and command frames.
func ValidateCommandTable(content []byte) error {
	if len(content) < machHeader64Size {
		return &Rejection{Code: TruncatedHeader, Detail: fmt.Sprintf("need 32 bytes, have %d", len(content))}
	}
	byteOrder := commandTableByteOrder(content)
	header := decodeMachHeader(content, byteOrder)
	if err := ValidateHeader(header, byteOrder); err != nil {
		return err
	}
	available := uint64(len(content) - machHeader64Size)
	if uint64(header.Cmdsz) > available {
		return &Rejection{Code: CommandTableOutsideArtifact, Detail: fmt.Sprintf("offset=32 cmdsz=%d exceeds artifact size=%d", header.Cmdsz, len(content))}
	}
	if header.Ncmd > header.Cmdsz/loadCommandHeaderSize {
		minimum := uint64(header.Ncmd) * loadCommandHeaderSize
		return &Rejection{Code: ImpossibleCommandCount, Detail: fmt.Sprintf("ncmd=%d cmdsz=%d requires at least %d bytes", header.Ncmd, header.Cmdsz, minimum)}
	}
	table := content[machHeader64Size:]
	table = table[:int(header.Cmdsz)]
	return validateCommandFrames(table, header.Ncmd, byteOrder)
}
