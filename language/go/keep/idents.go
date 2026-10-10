// Purpose: picks the names the keep file uses: the usual ones when the
// package does not have them, else the first numbered variant it does not.
// Never:   guesses that a name is free; it is checked against the package.
package keep

import "strconv"

// keepIdents are the three names the keep file takes: the name its import of
// the testing package goes by, its list of functions, and its test.
type keepIdents struct{ testing, variable, test string }

func identsFor(taken map[string]bool) keepIdents {
	return keepIdents{
		testing:  freeIdent("testing", taken),
		variable: freeIdent("hyperrayKeep", taken),
		test:     freeIdent("TestHyperrayKeep", taken),
	}
}

// freeIdent is base when the package has no such name, else the first of
// base1, base2, ... it does not have. The same package gives the same name
// every run, so the same project gives the same file.
func freeIdent(base string, taken map[string]bool) string {
	name := base
	for count := 1; taken[name]; count++ {
		name = base + strconv.Itoa(count)
	}
	return name
}

// importLine is the keep file's import of the testing package: plain when
// the package has no `testing` of its own, else under the free name.
func (idents keepIdents) importLine() string {
	if idents.testing == "testing" {
		return `"testing"`
	}
	return idents.testing + ` "testing"`
}
