// Variables identify finite state and nondeterministic input bit vectors.
// A zero-width or malformed variable never enters a step relation.
package circuit

// Variable is one named finite bit vector.
type Variable struct {
	name  string
	width uint16
}

// NewVariable builds a validated finite bit-vector variable.
func NewVariable(name string, width uint16) (Variable, error) {
	if err := identifierError(name, "variable.name"); err != nil {
		return Variable{}, err
	}
	if width == 0 {
		return Variable{}, engineError("zero_width", name)
	}
	return Variable{name: name, width: width}, nil
}

// Name returns the declared variable name.
func (variable Variable) Name() string {
	return variable.name
}

// Width returns the declared bit width.
func (variable Variable) Width() uint16 {
	return variable.width
}

func variableError(variable Variable, field string) error {
	if err := identifierError(variable.name, field); err != nil {
		return err
	}
	if variable.width == 0 {
		return engineError("zero_width", variable.name)
	}
	return nil
}
