// This file validates the bounded ordinary symbol table.
// It must reject unresolved, indirect, and out-of-range symbols.
package arm64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

func validateSymbolTable(command, content []byte, sectionCount int, order binary.ByteOrder, index uint32) error {
	symoff := uint64(order.Uint32(command[8:12]))
	symbolCount := uint64(order.Uint32(command[12:16]))
	stroff := uint64(order.Uint32(command[16:20]))
	stringSize := uint64(order.Uint32(command[20:24]))
	symbolBytes, err := boundedTable(content, symoff, symbolCount, nlist64Size, "symbols", index)
	if err != nil {
		return err
	}
	strings, err := boundedTable(content, stroff, 1, stringSize, "strings", index)
	if err != nil {
		return err
	}
	for offset := uint64(0); offset < symbolCount; offset++ {
		entry := symbolBytes[offset*nlist64Size : (offset+1)*nlist64Size]
		if err := validateSymbol(entry, strings, sectionCount, order, index, offset); err != nil {
			return err
		}
	}
	return nil
}

func boundedTable(content []byte, offset, count, width uint64, kind string, index uint32) ([]byte, error) {
	if width != 0 && count > math.MaxUint64/width {
		return nil, invalidSymbolTable(index, "%s table size overflows", kind)
	}
	length := count * width
	if offset > uint64(len(content)) || length > uint64(len(content))-offset {
		return nil, invalidSymbolTable(index, "%s range exceeds artifact", kind)
	}
	return content[int(offset):int(offset+length)], nil
}

func validateSymbol(entry, strings []byte, sectionCount int, order binary.ByteOrder, commandIndex uint32, symbolIndex uint64) error {
	nameOffset := order.Uint32(entry[0:4])
	if uint64(nameOffset) >= uint64(len(strings)) || bytes.IndexByte(strings[nameOffset:], 0) < 0 {
		return invalidSymbolTable(commandIndex, "symbol[%d] name is not terminated", symbolIndex)
	}
	kind := entry[4] & nTypeMask
	if kind == nUndefined || kind == nPrebound {
		// A symbol another library defines states nothing about the bytes
		// in this file. Whether the function under proof reaches it is
		// decided by ReadsRewrittenAddress.
		return nil
	}
	if kind == nIndirect {
		return &Rejection{Code: IndirectSymbol, Detail: fmt.Sprintf("command[%d] symbol[%d] is indirect", commandIndex, symbolIndex)}
	}
	if kind == nSection && (entry[5] == 0 || int(entry[5]) > sectionCount) {
		return &Rejection{Code: InvalidSymbolSection, Detail: fmt.Sprintf("command[%d] symbol[%d] section=%d", commandIndex, symbolIndex, entry[5])}
	}
	if entry[4]&nDebugMask == 0 && kind != nAbsolute && kind != nSection {
		return &Rejection{Code: UnsupportedSymbolType, Detail: fmt.Sprintf("command[%d] symbol[%d] type=0x%x", commandIndex, symbolIndex, entry[4])}
	}
	return nil
}

func invalidSymbolTable(index uint32, format string, values ...any) error {
	return &Rejection{Code: InvalidSymbolTable, Detail: fmt.Sprintf("command[%d]: %s", index, fmt.Sprintf(format, values...))}
}
