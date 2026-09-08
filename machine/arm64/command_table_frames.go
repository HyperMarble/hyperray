// This file validates command prefixes and frame bounds only.
// It must not parse unknown command payloads or assign execution meaning.
package arm64

import (
	"encoding/binary"
	"fmt"
)

func validateCommandFrames(table []byte, commandCount uint32, byteOrder binary.ByteOrder) error {
	cursor := uint64(0)
	for commandIndex := uint32(0); commandIndex < commandCount; commandIndex++ {
		prefix := table[int(cursor):]
		if len(prefix) < loadCommandHeaderSize {
			return &Rejection{Code: TruncatedCommandPrefix, Detail: fmt.Sprintf("command[%d] offset=%d: need 8-byte prefix, have %d bytes", commandIndex, machHeader64Size+cursor, len(prefix))}
		}
		commandSize := byteOrder.Uint32(prefix[4:8])
		if commandSize < loadCommandHeaderSize {
			return &Rejection{Code: InvalidCommandSize, Detail: fmt.Sprintf("command[%d] offset=%d: cmdsize=%d is less than 8", commandIndex, machHeader64Size+cursor, commandSize)}
		}
		if commandSize%loadCommandHeaderSize != 0 {
			return &Rejection{Code: InvalidCommandAlignment, Detail: fmt.Sprintf("command[%d] offset=%d: cmdsize=%d is not 8-byte aligned", commandIndex, machHeader64Size+cursor, commandSize)}
		}
		if uint64(commandSize) > uint64(len(prefix)) {
			return &Rejection{Code: CommandOutsideTable, Detail: fmt.Sprintf("command[%d] offset=%d: cmdsize=%d exceeds remaining=%d", commandIndex, machHeader64Size+cursor, commandSize, len(prefix))}
		}
		cursor += uint64(commandSize)
	}
	if int(cursor) != len(table) {
		if commandCount == 0 {
			return &Rejection{Code: TrailingCommandBytes, Detail: fmt.Sprintf("after command table offset=%d: cursor=%d cmdsz=%d", machHeader64Size+cursor, cursor, len(table))}
		}
		return &Rejection{Code: TrailingCommandBytes, Detail: fmt.Sprintf("after command[%d] offset=%d: cursor=%d cmdsz=%d", commandCount-1, machHeader64Size+cursor, cursor, len(table))}
	}
	return nil
}
