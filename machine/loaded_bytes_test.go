// These tests compare every recorded memory byte with ELF program data.
// They must include file-backed bytes and zero-filled memory bytes.
package machine_test

import (
	"bytes"
	"debug/elf"
	"sort"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestLoadRecordsEveryLoadableByte(t *testing.T) {
	image, content := acceptedFixture(t, "rv64-lp64d-static.elf")
	file, err := elf.NewFile(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("elf.NewFile() error = %v", err)
	}
	programs := loadPrograms(file)
	loadedIndex := 0
	for _, program := range programs {
		loadedIndex = compareProgramBytes(t, content, program, image.LoadedBytes, loadedIndex)
	}
	if loadedIndex != len(image.LoadedBytes) {
		t.Errorf("compared %d loaded bytes, recorded %d", loadedIndex, len(image.LoadedBytes))
	}
}

func loadPrograms(file *elf.File) []*elf.Prog {
	programs := make([]*elf.Prog, 0)
	for _, program := range file.Progs {
		if program.Type == elf.PT_LOAD {
			programs = append(programs, program)
		}
	}
	sort.Slice(programs, func(left, right int) bool {
		return programs[left].Vaddr < programs[right].Vaddr
	})
	return programs
}

func compareProgramBytes(t *testing.T, content []byte, program *elf.Prog, loaded []machine.LoadedByte, start int) int {
	t.Helper()
	permissions := machine.Permissions{
		Readable:   program.Flags&elf.PF_R != 0,
		Writable:   program.Flags&elf.PF_W != 0,
		Executable: program.Flags&elf.PF_X != 0,
	}
	for offset := uint64(0); offset < program.Memsz; offset++ {
		index := start + int(offset)
		if index >= len(loaded) {
			t.Fatalf("program byte %d exceeds %d loaded bytes", index, len(loaded))
		}
		wantValue := expectedByte(content, program, offset)
		wantAddress := program.Vaddr + offset
		if loaded[index].Address != wantAddress || loaded[index].Value != wantValue {
			t.Errorf("LoadedBytes[%d] = %#v, want address %#x value %#x", index, loaded[index], wantAddress, wantValue)
		}
		if loaded[index].Permissions != permissions {
			t.Errorf("LoadedBytes[%d].Permissions = %#v, want %#v", index, loaded[index].Permissions, permissions)
		}
	}
	return start + int(program.Memsz)
}

func expectedByte(content []byte, program *elf.Prog, offset uint64) byte {
	if offset >= program.Filesz {
		return 0
	}
	return content[program.Off+offset]
}
