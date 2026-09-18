// Program currency checks reject changed or mismatched generated artifacts.
// A zero or caller-mismatched value cannot authorize a verdict.
package isla

func (program Program) current(request VerificationRequest) error {
	if len(program.content) == 0 || !validDigest(program.digest) || !validDigest(program.imageDigest) {
		return engineError(InvalidInput, "generated program", "empty or invalid identity")
	}
	if contentDigest(program.content) != program.digest {
		return engineError(ArtifactChanged, "generated program", "content digest differs")
	}
	if program.threadIdentity != "" && threadEntryIdentity(program.threadEntries) != program.threadIdentity {
		return engineError(ArtifactChanged, "generated program", "thread identity differs")
	}
	if request.query.program.digest != program.digest {
		return engineError(CoverageMismatch, "generated program", "request digest differs")
	}
	if uint64(len(program.instructions)) != program.instructionCount || program.instructionCount == 0 {
		return engineError(CoverageMismatch, "generated program", "instruction inventory differs")
	}
	return nil
}
