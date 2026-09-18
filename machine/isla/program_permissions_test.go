// Adjacent load regions retain distinct memory permissions in generated input.
// Section grouping must not turn writable bytes into constants.
package isla

import (
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestLoadedSectionsSplitPermissions(t *testing.T) {
	readonly := machine.Permissions{Readable: true}
	writable := machine.Permissions{Readable: true, Writable: true}
	loaded := []machine.LoadedByte{
		{Address: 16, Value: 11, Permissions: readonly},
		{Address: 17, Value: 23, Permissions: writable},
		{Address: 18, Value: 29, Permissions: writable},
	}
	sections := loadedSections(loaded)
	if len(sections) != 2 || sections[0].permissions.Writable || !sections[1].permissions.Writable {
		t.Fatalf("permission boundaries changed: %#v", sections)
	}
	if sections[1].address != 17 || len(sections[1].bytes) != 2 {
		t.Errorf("writable bytes changed: %#v", sections[1])
	}
	output := newLimitedBuffer(1024)
	writeSections(output, sections)
	if !strings.Contains(output.String(), "writable = true") || !strings.Contains(output.String(), "writable = false") {
		t.Errorf("missing permission metadata: %s", output.String())
	}
}

func TestGeneratedQueryRequiresInitializedMemory(t *testing.T) {
	query := Request{executableProgram: true}
	semantic := VerificationRequest{query: query}
	for _, arguments := range [][]string{query.arguments(), semantic.semanticArguments()} {
		if !strings.Contains(strings.Join(arguments, " "), "--initialized-memory") {
			t.Errorf("missing memory capability: %v", arguments)
		}
	}
}
