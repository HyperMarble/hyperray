// Verifier construction joins identified semantic and solver tools.
// Tools from different reported Isla versions cannot form one proof result.
package isla

// Verifier runs same-program semantic coverage and solver checks.
type Verifier struct {
	solver    Engine
	semantics SemanticEngine
}

// NewVerifier accepts two current Isla tools from one reported version.
func NewVerifier(solver Engine, semantics SemanticEngine) (Verifier, error) {
	if err := solver.current(); err != nil {
		return Verifier{}, err
	}
	if err := semantics.current(); err != nil {
		return Verifier{}, err
	}
	if solver.identity.Version != semantics.identity.Version {
		return Verifier{}, engineError(ReleaseMismatch, "Isla tools", "versions differ")
	}
	return Verifier{solver: solver, semantics: semantics}, nil
}
