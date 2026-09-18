// Semantic arguments use the same model, configuration, and program as the query.
// They select the complete human event-tree format with finite limits.
package isla

import "strconv"

func (request VerificationRequest) semanticArguments() []string {
	arguments := []string{
		"-T", strconv.FormatUint(request.threadLimit, 10),
		"-A", PreparsedArchitecture(request.query.architecture.path),
		"-C", request.query.configuration.path,
		"--memory", strconv.FormatUint(request.memoryLimitMB, 10),
		"--pc-limit", strconv.FormatUint(request.query.pcVisitLimit, 10),
		"-f", "human", request.query.program.path,
	}
	if request.query.executableProgram {
		arguments = append(arguments, "--executable-entry", "--initialized-memory")
	}
	return append(arguments, request.query.executionArguments()...)
}
