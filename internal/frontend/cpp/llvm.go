package cpp

import (
	"fmt"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// validateLLVMOperation makes the compiler IR, rather than AST syntax, a
// mandatory semantic-evidence boundary. The AST supplies typed source spans;
// this check requires the exact mangled definition and the corresponding
// compiler control/terminal operations to exist in the LLVM module emitted by
// the pinned compilation-database invocation.
func (l *lowerer) validateLLVMOperation(node *astNode, statements []semanticir.Statement) error {
	if strings.TrimSpace(l.llvmIR) == "" {
		return fmt.Errorf("LLVM module is empty")
	}
	symbol := node.MangledName
	if symbol == "" {
		symbol = node.Name
	}
	body, ok := llvmFunctionBody(l.llvmIR, symbol)
	if !ok && strings.Contains(l.llvmIR, "target triple = \"arm64-apple-") && strings.HasPrefix(symbol, "__Z") {
		// Mach-O's assembler symbol has one platform underscore that is not
		// present in LLVM IR global identifiers.
		body, ok = llvmFunctionBody(l.llvmIR, symbol[1:])
	}
	if !ok {
		return fmt.Errorf("definition for exact compiler symbol %q is absent", symbol)
	}
	shape := statementShape(statements)
	if shape.terminals > 0 && !strings.Contains(body, " ret ") && !strings.Contains(body, "\nret ") && !strings.Contains(body, " unreachable") {
		return fmt.Errorf("translated terminal has no LLVM ret/unreachable instruction")
	}
	if shape.branches > 0 && !strings.Contains(body, " br i1 ") && !strings.Contains(body, " switch ") && !strings.Contains(body, " select ") {
		return fmt.Errorf("translated branch has no LLVM branch/switch/select semantics")
	}
	if shape.raises > 0 && !strings.Contains(body, "@__cxa_throw") {
		return fmt.Errorf("translated C++ throw has no LLVM ABI throw operation")
	}
	if shape.writes > 0 && !strings.Contains(body, " store ") {
		return fmt.Errorf("translated state write has no LLVM store instruction")
	}
	return nil
}

type llvmStatementShape struct {
	branches  int
	terminals int
	raises    int
	writes    int
}

func statementShape(statements []semanticir.Statement) llvmStatementShape {
	var shape llvmStatementShape
	for _, statement := range statements {
		switch statement.Kind {
		case semanticir.StmtBranch:
			shape.branches++
		case semanticir.StmtReturn:
			shape.terminals++
		case semanticir.StmtRaise:
			shape.terminals++
			shape.raises++
		}
		for _, effect := range statement.Effects {
			if effect.Kind == semanticir.EffectWrite {
				shape.writes++
			}
		}
		child := statementShape(statement.Then)
		shape.branches += child.branches
		shape.terminals += child.terminals
		shape.raises += child.raises
		shape.writes += child.writes
		child = statementShape(statement.Else)
		shape.branches += child.branches
		shape.terminals += child.terminals
		shape.raises += child.raises
		shape.writes += child.writes
	}
	return shape
}

func llvmFunctionBody(module, symbol string) (string, bool) {
	if symbol == "" {
		return "", false
	}
	lines := strings.Split(module, "\n")
	needle := "@" + symbol + "("
	quotedNeedle := "@\"" + symbol + "\"("
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "define ") || !strings.Contains(line, needle) && !strings.Contains(line, quotedNeedle) {
			continue
		}
		var body strings.Builder
		depth := 0
		started := false
		for _, candidate := range lines[index:] {
			body.WriteString(candidate)
			body.WriteByte('\n')
			depth += strings.Count(candidate, "{")
			if strings.Contains(candidate, "{") {
				started = true
			}
			depth -= strings.Count(candidate, "}")
			if started && depth == 0 {
				return body.String(), true
			}
		}
		return "", false
	}
	return "", false
}
