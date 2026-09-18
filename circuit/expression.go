// Expressions hold typed bit-vector and Boolean step semantics.
// Callers cannot inject unparsed SMT text into these expressions.
package circuit

type bitVectorOperation uint8

const (
	readVariable bitVectorOperation = iota + 1
	subtractValues
)

type bitVectorNode struct {
	operation bitVectorOperation
	variable  Variable
	left      *bitVectorNode
	right     *bitVectorNode
}

// BitVectorExpression is an immutable typed bit-vector expression.
type BitVectorExpression struct {
	node *bitVectorNode
}

// Read returns an expression that reads one declared variable.
func Read(variable Variable) BitVectorExpression {
	return BitVectorExpression{node: &bitVectorNode{operation: readVariable, variable: variable}}
}

// Subtract returns fixed-width modular subtraction.
func Subtract(left BitVectorExpression, right BitVectorExpression) (BitVectorExpression, error) {
	leftWidth, err := expressionWidth(left)
	if err != nil {
		return BitVectorExpression{}, err
	}
	rightWidth, err := expressionWidth(right)
	if err != nil {
		return BitVectorExpression{}, err
	}
	if leftWidth != rightWidth {
		return BitVectorExpression{}, engineError("width_mismatch", "subtract")
	}
	node := &bitVectorNode{operation: subtractValues, left: left.node, right: right.node}
	return BitVectorExpression{node: node}, nil
}
