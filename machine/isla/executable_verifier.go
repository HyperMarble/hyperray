// Executable verifier construction joins all trusted Isla operations.
// A mixed release cannot enter executable verification.
package isla

// ExecutableVerifier checks static semantics and the whole-program query.
type ExecutableVerifier struct {
	verifier   Verifier
	footprints FootprintEngine
	release    FootprintRelease
	capability *ARM64ExecutionCapability
}

// NewExecutableVerifier accepts one current and version-matched Isla release.
func NewExecutableVerifier(verifier Verifier, footprints FootprintEngine, release FootprintRelease) (ExecutableVerifier, error) {
	if err := verifier.current(); err != nil {
		return ExecutableVerifier{}, err
	}
	if err := release.current(); err != nil {
		return ExecutableVerifier{}, err
	}
	if err := release.matches(footprints); err != nil {
		return ExecutableVerifier{}, err
	}
	if verifier.solver.identity.Version != footprints.identity.Version {
		return ExecutableVerifier{}, engineError(ReleaseMismatch, "Isla tools", "versions differ")
	}
	return ExecutableVerifier{verifier: verifier, footprints: footprints, release: release}, nil
}
