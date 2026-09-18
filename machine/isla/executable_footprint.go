// Executable footprint requests inherit the bound program profile.
// The ARM register name changes only through a validated capability.
package isla

func (engine ExecutableVerifier) newFootprintRequest(program Program, limits ExecutableLimits) (FootprintRequest, error) {
	pcRegister := "PC"
	if engine.capability != nil {
		pcRegister = engine.capability.pcRegister
	}
	return newFootprintRequest(engine.release, program.instructions, limits.ThreadLimit, limits.TimeLimitSeconds, limits.MaximumOutputBytes, pcRegister)
}
