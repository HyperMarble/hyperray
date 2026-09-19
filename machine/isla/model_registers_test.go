// The model register reader must return the model's own names, decoded
// exactly as Sail encoded them, and must refuse a file with no register.
package isla_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestDecodeSailNameReversesEveryEscape(t *testing.T) {
	// Each pair was produced by running the character through Sail's
	// Util.zchar, transcribed from sail/src/lib/util.ml.
	cases := map[string]string{
		"z__monomorphizze_reads": "__monomorphize_reads",
		"zPSTATE":                "PSTATE",
		"zR30":                   "R30",
		"z_PC":                   "_PC",
		"zz3name":                "#name",
		"zazEb":                  "a.b",
		"zazGb":                  "a:b",
		"zazNb":                  "a[b",
		"zazSb":                  "a`b",
		"zazUb":                  "a|b",
	}
	for encoded, want := range cases {
		got, err := isla.DecodeSailName(encoded)
		if err != nil {
			t.Fatalf("DecodeSailName(%q) error = %v", encoded, err)
		}
		if got != want {
			t.Fatalf("DecodeSailName(%q) = %q, want %q", encoded, got, want)
		}
	}
}

func TestDecodeSailNameRejectsMalformedInput(t *testing.T) {
	for _, encoded := range []string{"PSTATE", "zabcz", "zazXb"} {
		if _, err := isla.DecodeSailName(encoded); err == nil {
			t.Fatalf("DecodeSailName(%q) accepted malformed input", encoded)
		}
	}
}

func TestModelRegisterNamesReadsDeclarations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.ir")
	content := "val f : %unit -> %unit\nregister zPSTATE : %struct zProcState\nregister z__v81_implemented : %bool\nregister zR30 : %bv64\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := isla.ModelRegisterNames(path)
	if err != nil {
		t.Fatalf("ModelRegisterNames() error = %v", err)
	}
	want := []string{"PSTATE", "__v81_implemented", "R30"}
	if len(got) != len(want) {
		t.Fatalf("ModelRegisterNames() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("ModelRegisterNames()[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestModelRegisterNamesRejectsModelWithoutRegisters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.ir")
	if err := os.WriteFile(path, []byte("val f : %unit -> %unit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := isla.ModelRegisterNames(path); err == nil {
		t.Fatal("ModelRegisterNames() accepted a model with no register")
	}
}

func TestARM64MemoryObservationsRejectModelRegisterName(t *testing.T) {
	content, start, end := arm64ProgramFixture(t)
	boundary := arm64ProgramBoundary(start, end)
	boundary.NativeRegisterNames = []string{"v9Ap4_IMPLEMENTED"}
	boundary.MemoryObservations = []isla.MemoryObservation{
		{Name: "v9Ap4_IMPLEMENTED", Address: start, Bytes: 1},
	}
	if _, err := isla.BuildARM64Program(content, 32768, boundary); err == nil {
		t.Fatal("BuildARM64Program() accepted an observation named after a model register")
	}
}
