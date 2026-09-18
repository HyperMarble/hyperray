// This file admits only source-backed Mach-O load commands.
// It must reject every command that can impose hidden runtime work.
package arm64

import (
	"debug/macho"
	"encoding/binary"
	"fmt"
)

func validateLoadCommands(content []byte, sectionCount int) error {
	order := commandTableByteOrder(content)
	header := decodeMachHeader(content, order)
	table := content[machHeader64Size : machHeader64Size+int(header.Cmdsz)]
	cursor := uint64(0)
	for index := uint32(0); index < header.Ncmd; index++ {
		command := table[int(cursor):]
		commandSize := order.Uint32(command[4:8])
		commandID := macho.LoadCmd(order.Uint32(command[0:4]))
		if err := validateLoadCommand(commandID, command, commandSize, content, sectionCount, order, index); err != nil {
			return err
		}
		cursor += uint64(commandSize)
	}
	return nil
}

func validateLoadCommand(id macho.LoadCmd, command []byte, size uint32, content []byte, sectionCount int, order binary.ByteOrder, index uint32) error {
	switch id {
	case macho.LoadCmdSegment64:
		return nil
	case macho.LoadCmdSymtab:
		if size != 24 {
			return invalidCommand(index, id, size, "requires 24-byte framing")
		}
		return validateSymbolTable(command, content, sectionCount, order, index)
	case loadCmdUUID:
		if size != 24 {
			return invalidCommand(index, id, size, "requires 24-byte framing")
		}
		return nil
	case loadCmdSourceVersion:
		if size != 16 {
			return invalidCommand(index, id, size, "requires 16-byte framing")
		}
		return nil
	case loadCmdCodeSignature, loadCmdFunctionStarts, loadCmdDataInCode,
		loadCmdBuildVersion:
		return nil
	case loadCmdDyldInfoOnly, loadCmdDynamicSymtab, loadCmdLoadDylib,
		loadCmdLoadDylinker, loadCmdMain:
		return nil
	case macho.LoadCmdUnixThread:
		return validateThreadCommand(command, size, order, index)
	default:
		return &Rejection{Code: UnsupportedLoadCommand, Detail: fmt.Sprintf("command[%d]: id=0x%x", index, uint32(id))}
	}
}

func invalidCommand(index uint32, id macho.LoadCmd, size uint32, reason string) error {
	return &Rejection{Code: InvalidLoadCommand, Detail: fmt.Sprintf("command[%d] id=0x%x cmdsize=%d %s", index, uint32(id), size, reason)}
}

func validateThreadCommand(command []byte, size uint32, order binary.ByteOrder, index uint32) error {
	if size != threadCommand64Size {
		return invalidCommand(index, macho.LoadCmdUnixThread, size, "requires ARM64 thread framing")
	}
	flavor := order.Uint32(command[8:12])
	count := order.Uint32(command[12:16])
	if flavor != armThreadState64 || count != armThreadState64Count {
		return &Rejection{Code: InvalidThreadState, Detail: fmt.Sprintf("command[%d]: flavor=%d count=%d", index, flavor, count)}
	}
	return nil
}
