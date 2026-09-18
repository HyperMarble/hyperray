// Boolean expressions hold comparisons over finite bit-vector values.
// They never use host-language integer comparison as circuit semantics.
package circuit

type comparisonOperation uint8

const (
	unsignedLessThan comparisonOperation = iota + 1
	unsignedLessOrEqual
)

// BooleanExpression is an immutable typed Boolean expression.
type BooleanExpression struct {
	operation comparisonOperation
	left      BitVectorExpression
	right     BitVectorExpression
}

// UnsignedLessThan compares two equal-width bit vectors.
func UnsignedLessThan(left BitVectorExpression, right BitVectorExpression) (BooleanExpression, error) {
	return comparison(unsignedLessThan, left, right)
}

// UnsignedLessOrEqual compares two equal-width bit vectors.
func UnsignedLessOrEqual(left BitVectorExpression, right BitVectorExpression) (BooleanExpression, error) {
	return comparison(unsignedLessOrEqual, left, right)
}

func comparison(
	operation comparisonOperation,
	left BitVectorExpression,
	right BitVectorExpression,
) (BooleanExpression, error) {
	leftWidth, err := expressionWidth(left)
	if err != nil {
		return BooleanExpression{}, err
	}
	rightWidth, err := expressionWidth(right)
	if err != nil {
		return BooleanExpression{}, err
	}
	if leftWidth != rightWidth {
		return BooleanExpression{}, engineError("width_mismatch", "comparison")
	}
	return BooleanExpression{operation: operation, left: left, right: right}, nil
}
