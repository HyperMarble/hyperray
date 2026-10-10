// Purpose: a function stored in a package-level variable that nothing in
// the package uses still has its machine code in the test program.
// Never:   keeps only declared functions and lets a function value in a
// variable be dropped.
package goadapter_test

import (
	"strings"
	"testing"

	goadapter "github.com/HyperMarble/hyperray/language/go"
)

const functionsInVariables = `package lib

var Hook = func(n int) int { return n * 7 }

var (
	hidden = func(n int) int { return n * 3 }
	first, second = func() int { return 1 }, func() int { return 2 }
)

var _ = func() int { return 9 }

const Limit = 4

var table = map[string]func() int{"a": func() int { return 5 }}

func Plain(n int) int { return n + 1 }
`

func TestFunctionsStoredInPackageVariablesKeepTheirMachineCode(t *testing.T) {
	root, out := writeModule(t, map[string]string{"lib/lib.go": functionsInVariables})
	record := built(t, root, out, goadapter.Choice{})
	if len(record.NotBuilt) > 0 {
		t.Fatalf("not built: %+v", record.NotBuilt)
	}
	stored := 0
	for _, name := range symbols(t, record.Artifacts[0].File.Path) {
		if strings.Contains(name, "/lib.init.func") {
			stored++
		}
	}
	if stored < 5 {
		t.Fatalf("5 functions are stored in named package variables, %d have machine code", stored)
	}
}
