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
	// Apple SDK mach-o/loader.h. These state that a dynamic loader fills in
	// addresses before the program runs. The sections it fills are named by
	// RewrittenRanges, and a function that reaches one is refused there.
	loadCmdDyldInfoOnly   macho.LoadCmd = 0x80000022
	loadCmdChainedFixups  macho.LoadCmd = 0x80000034
	loadCmdExportsTrie    macho.LoadCmd = 0x80000033
	loadCmdDynamicSymtab  macho.LoadCmd = 0x0b
	loadCmdLoadDylib      macho.LoadCmd = 0x0c
	loadCmdLoadDylinker   macho.LoadCmd = 0x0e
	loadCmdMain           macho.LoadCmd = 0x80000028
	armThreadState64      uint32        = 6
	armThreadState64Count uint32        = 68
	threadCommand64Size   uint32        = 8 + 8 + armThreadState64Count*4
)
