// Program sections retain pre-entry instructions and original memory bytes.
// An entry offset must never shift or omit a loaded region.
package isla

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestProgramSectionsRetainPreEntryBytes(t *testing.T) {
	content, err := os.ReadFile("../../fixtures/machine/rv64-lp64d-static.elf")
	if err != nil {
		t.Fatal(err)
	}
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	image.EntryAddress = image.Instructions[1].Address
	layout, err := layoutProgram(image)
	if err != nil {
		t.Fatal(err)
	}
	var reconstructed []machine.LoadedByte
	for _, section := range layout.sections {
		for offset, value := range section.bytes {
			reconstructed = append(reconstructed, machine.LoadedByte{
				Address: section.address + uint64(offset), Value: value, Permissions: section.permissions,
			})
		}
	}
	if !slices.Equal(reconstructed, image.LoadedBytes) {
		t.Error("section bytes differ from the loaded ELF")
	}
}

func TestExecutableEntryCapabilityArguments(t *testing.T) {
	query := Request{executableProgram: true}
	semantic := VerificationRequest{query: query}
	for _, arguments := range [][]string{query.arguments(), semantic.semanticArguments()} {
		if !slices.Contains(arguments, "--executable-entry") {
			t.Errorf("missing required tool capability: %v", arguments)
		}
	}
	legacy := VerificationRequest{}
	if strings.Contains(strings.Join(legacy.semanticArguments(), " "), "--executable-entry") {
		t.Error("legacy query unexpectedly requires the executable extension")
	}
}
