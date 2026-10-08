// Purpose: reads the symbol names of a built file with the reader for this
// system's file format, so a test can see which functions have machine code.
// Never:   reads an ELF file with the Mach-O reader or the other way round.
package goadapter_test

import (
	"debug/elf"
	"debug/macho"
	"runtime"
	"testing"
)

// symbols lists the names of every function that has its own machine code
// in the built file, read with the reader for this system's file format.
func symbols(t *testing.T, path string) []string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		return machoSymbols(t, path)
	}
	return elfSymbols(t, path)
}

func machoSymbols(t *testing.T, path string) []string {
	t.Helper()
	file, err := macho.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	names := []string{}
	for _, symbol := range file.Symtab.Syms {
		names = append(names, symbol.Name)
	}
	return names
}

func elfSymbols(t *testing.T, path string) []string {
	t.Helper()
	file, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	found, err := file.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, symbol := range found {
		names = append(names, symbol.Name)
	}
	return names
}
