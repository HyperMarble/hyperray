// Executable input validation selects only a supported profile and release.
// It must run before any footprint or solver process is started.
package isla

import (
	"context"

	"github.com/HyperMarble/hyperray/machine"
)

func (engine ExecutableVerifier) validateExecutableInput(ctx context.Context, request VerificationRequest, program Program) error {
	if ctx == nil {
		return engineError(InvalidInput, "context", "nil")
	}
	if engine.capability == nil && program.profile != "" && program.profile != machine.ProfileName {
		return engineError(UnsupportedProfile, "program profile", program.profile+" has no native verifier capability")
	}
	if engine.capability == nil {
		return nil
	}
	if err := engine.capability.current(); err != nil {
		return err
	}
	if err := engine.capability.validateProgram(program); err != nil {
		return err
	}
	return engine.capability.validateRequest(request)
}

func configureExecutableRequest(request *VerificationRequest, program Program, limits ExecutableLimits) {
	request.query.executableProgram = true
	request.query.workerCount = limits.ThreadLimit
	request.query.sequentialMemory = program.memoryProfile == SequentialMemory
	request.query.typedInitialState = program.typedInitialState
	request.query.forbiddenModelCalls = program.forbiddenModelCalls
}
