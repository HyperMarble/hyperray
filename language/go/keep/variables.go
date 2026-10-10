// Purpose: names every package-level variable, so the keep file can hold
// its address. A function stored in a variable (`var Hook = func...`) has
// machine code only while that variable is reachable, and the linker drops
// a variable nothing uses.
// Never:   names a constant or the blank name; neither has an address.
package keep

import (
	"go/ast"
	"go/token"
)

// variableRefs is `&name` for each package-level variable the declaration
// brings in. The address keeps the variable without copying its value.
func variableRefs(decl ast.Decl) []string {
	general, ok := decl.(*ast.GenDecl)
	if !ok || general.Tok != token.VAR {
		return nil
	}
	refs := []string{}
	for _, name := range specNames(general.Specs) {
		if name != "_" {
			refs = append(refs, "&"+name)
		}
	}
	return refs
}
