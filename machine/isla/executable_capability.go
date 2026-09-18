// Capability-aware verifier construction keeps ARM execution separate from RV.
// The generic footprint release remains unchanged for existing callers.
package isla

// NewExecutableVerifierWithCapability binds an ARM capability to all operations.
func NewExecutableVerifierWithCapability(verifier Verifier, footprints FootprintEngine, release FootprintRelease, capability ARM64ExecutionCapability) (ExecutableVerifier, error) {
	if err := capability.current(); err != nil {
		return ExecutableVerifier{}, err
	}
	if err := capability.matchesRelease(release); err != nil {
		return ExecutableVerifier{}, err
	}
	if err := capability.matchesVerifier(verifier, footprints); err != nil {
		return ExecutableVerifier{}, err
	}
	engine, err := NewExecutableVerifier(verifier, footprints, release)
	if err != nil {
		return ExecutableVerifier{}, err
	}
	engine.capability = &capability
	return engine, nil
}
