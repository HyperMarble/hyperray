//go:build isla_integration && arm64_acceptance

// This helper checks the exact Mach-O section and text symbol inventory.
// It must not mix fixture provenance with engine construction.
package isla_test

import (
	"debug/macho"
	"testing"
)

func arm64TextSection(t *testing.T, file *macho.File) *macho.Section {
	t.Helper()
	matches := make([]*macho.Section, 0, 1)
	for _, section := range file.Sections {
		if section.Seg == "__TEXT" && section.Name == "__text" {
			matches = append(matches, section)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("found %d __TEXT,__text sections, want 1", len(matches))
	}
	return matches[0]
}

func arm64TextSymbol(t *testing.T, file *macho.File, address uint64) {
	t.Helper()
	if file.Symtab == nil {
		t.Fatal("Mach-O has no symbol table")
	}
	textIndex := uint8(0)
	for index, section := range file.Sections {
		if section.Seg == "__TEXT" && section.Name == "__text" && section.Addr == address {
			textIndex = uint8(index + 1)
		}
	}
	if textIndex == 0 {
		t.Fatal("Mach-O text symbol has no matching section")
	}
	expected := map[string]uint64{
		"__mh_execute_header": 0x100000000,
		"_arm64_fixture":      address,
	}
	seen := make(map[string]bool, len(expected))
	for _, symbol := range file.Symtab.Syms {
		want, ok := expected[symbol.Name]
		if !ok || seen[symbol.Name] || symbol.Value != want || (symbol.Name == "_arm64_fixture" && symbol.Sect != textIndex) {
			t.Fatalf("unexpected __text symbol = %#v", symbol)
		}
		seen[symbol.Name] = true
	}
	if len(seen) != len(expected) || !seen["_arm64_fixture"] {
		t.Fatalf("Mach-O symbols = %d, want exact inventory of %d including _arm64_fixture", len(seen), len(expected))
	}
}
