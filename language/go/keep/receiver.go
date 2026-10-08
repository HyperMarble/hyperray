// Purpose: names a method's receiver the way Go code refers to it by
// value: `T` for a value receiver, `(*T)` for a pointer receiver, with
// any parentheses the spec allows around the type stripped first.
// Never:   names a generic receiver; Go has no expression for one method
// body of a generic type.
package keep

import "go/ast"

// receiverName is `T` or `(*T)`. The spec allows parentheses around any
// type, so `(T)`, `(*T)` and `*(T)` name the same receivers. A generic
// receiver `T[X]` has no name that refers to one method body.
func receiverName(expr ast.Expr) (string, bool) {
	switch typed := unparen(expr).(type) {
	case *ast.Ident:
		return typed.Name, true
	case *ast.StarExpr:
		ident, ok := unparen(typed.X).(*ast.Ident)
		if !ok {
			return "", false
		}
		return "(*" + ident.Name + ")", true
	}
	return "", false
}

// unparen is expr with every outer pair of parentheses removed.
func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}
