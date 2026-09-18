// Variable tests cover the public name and finite-width contract.
// They never infer valid SMT names from solver acceptance.
package circuit_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/circuit"
)

func TestRelationValidationRejectsInvalidVariables(t *testing.T) {
	_, err := circuit.NewVariable("", 8)
	requireEngineError(t, err, "empty_identifier", "variable.name")
	_, err = circuit.NewVariable("1balance", 8)
	requireEngineError(t, err, "invalid_identifier", "variable.name", "1balance")
	_, err = circuit.NewVariable("bad name", 8)
	requireEngineError(t, err, "invalid_identifier", "variable.name", "bad name")
	_, err = circuit.NewVariable("balance", 0)
	requireEngineError(t, err, "zero_width", "balance")
}

func TestVariablePublicValues(t *testing.T) {
	variable := mustVariable(t, "balance", 8)
	if variable.Name() != "balance" || variable.Width() != 8 {
		t.Errorf("variable = %q/%d, want balance/8", variable.Name(), variable.Width())
	}
}
