// A dynamic loader fills in stub and pointer sections before a program runs.
// A function that calls through them would be proved against bytes that
// never execute, so it is named rather than proved.
package isla

import (
	"github.com/HyperMarble/hyperray/machine"
	"github.com/HyperMarble/hyperray/machine/arm64"
)

func validateNoLoadTimeRewriting(content []byte, image machine.Image, boundary ARM64ProgramBoundary) error {
	ranges, err := arm64.RewrittenRanges(content)
	if err != nil {
		return engineError(InvalidInput, "ARM64 image", err.Error())
	}
	if len(ranges) == 0 {
		return nil
	}
	within := instructionsUnderProof(image.Instructions, boundary.FunctionStart, boundary.FunctionEnd)
	if err := arm64.ReadsRewrittenAddress(within, ranges); err != nil {
		return engineError(CoverageMismatch, "ARM64 function", err.Error())
	}
	return nil
}
