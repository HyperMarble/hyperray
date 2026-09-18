// Sequential program stages reuse Sail's complete memory-address partition.
// Isolated instruction analysis must retain its symbolic address domain.
package isla

func (request Request) executionArguments() []string {
	var arguments []string
	if request.sequentialMemory {
		arguments = []string{
			"--sequential-memory",
			"-I", "__monomorphize_reads = true",
			"--footprint-config", request.configuration.path,
		}
	}
	if request.typedInitialState {
		arguments = append(arguments, "--typed-initial-state")
	}
	if len(request.forbiddenModelCalls) != 0 {
		arguments = append(arguments, "--forbidden-model-calls")
	}
	for _, name := range request.forbiddenModelCalls {
		arguments = append(arguments, "--trace-function", name)
	}
	return arguments
}
