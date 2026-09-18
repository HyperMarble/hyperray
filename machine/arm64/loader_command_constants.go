// This file records Mach-O command constants from Apple SDK layouts.
// It must not invent command IDs or thread-state sizes.
package arm64

import "debug/macho"

const (
	loadCmdUUID          macho.LoadCmd = 0x1b
	loadCmdSourceVersion macho.LoadCmd = 0x2a
	// Apple SDK mach-o/loader.h. These describe the file and do not place,
	// move or rewrite any byte in memory.
	loadCmdCodeSignature  macho.LoadCmd = 0x1d
	loadCmdFunctionStarts macho.LoadCmd = 0x26
	loadCmdDataInCode     macho.LoadCmd = 0x29
	loadCmdBuildVersion   macho.LoadCmd = 0x32
	armThreadState64      uint32        = 6
	armThreadState64Count uint32        = 68
	threadCommand64Size   uint32        = 8 + 8 + armThreadState64Count*4
)
