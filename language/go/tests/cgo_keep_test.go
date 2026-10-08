// Purpose: a Go function declared in a cgo file also gets its own machine code.
// Never:   count only GoFiles when Go lists cgo source separately.
package goadapter_test

import (
	"os/exec"
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const cgoSource = `package lib
/* int one(void) { return 1; } */
import "C"
func UnusedCgo() int { return int(C.one()) }
`

func TestUnusedCgoFunctionHasMachineCode(t *testing.T) {
	enabled, err := exec.Command("go", "env", "CGO_ENABLED").Output()
	if err != nil || strings.TrimSpace(string(enabled)) != "1" {
		t.Skip("cgo is unavailable")
	}
	root, out := writeModule(t, map[string]string{"lib/cgo.go": cgoSource})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.Artifacts) != 1 || !hasSymbol(symbols(t, record.Artifacts[0].File.Path), "lib.UnusedCgo") {
		t.Fatalf("cgo function has no retained body: %+v", record.Artifacts)
	}
}
