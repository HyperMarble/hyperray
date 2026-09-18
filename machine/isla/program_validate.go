// Program validation rejects ambiguous text and mismatched machine addresses.
// It keeps all resource bounds in caller-visible input.
package isla

import (
	"github.com/HyperMarble/hyperray/machine"
)

func validateProgramBoundary(image machine.Image, boundary ProgramBoundary) ([]ThreadEntry, error) {
	if err := validateForbiddenCalls(boundary); err != nil {
		return nil, err
	}
	if err := validateMemoryProfile(boundary.MemoryProfile); err != nil {
		return nil, err
	}
	if boundary.MaximumProgramBytes == 0 {
		return nil, engineError(InvalidInput, "program size", "limit is zero")
	}
	if !plainLine(boundary.Name) || !plainLine(boundary.NegatedAssertion) {
		return nil, engineError(InvalidInput, "program text", "name or assertion is not one printable line")
	}
	threads, err := validatedThreads(image, boundary)
	if err != nil {
		return nil, err
	}
	if err := validateInitialState(boundary, threads); err != nil {
		return nil, err
	}
	return threads, nil
}
