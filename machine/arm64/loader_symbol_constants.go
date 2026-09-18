// This file records Mach-O nlist values used by symbol admission.
// It must not treat unresolved runtime symbols as static metadata.
package arm64

const (
	nlist64Size uint64 = 16
	nTypeMask   byte   = 0x0e
	nUndefined  byte   = 0x00
	nAbsolute   byte   = 0x02
	nPrebound   byte   = 0x0c
	nIndirect   byte   = 0x0a
	nSection    byte   = 0x0e
	nDebugMask  byte   = 0xe0
)
