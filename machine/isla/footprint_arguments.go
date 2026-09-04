// Footprint arguments pass bytes and addresses as data to the Sail decoder.
// They contain no opcode or instruction-family table.
package isla

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/HyperMarble/hyperray/machine"
)

func (request FootprintRequest) arguments(instruction machine.Instruction) []string {
	initialPC := fmt.Sprintf("PC=0x%016x", instruction.Address)
	return []string{
		"-T", strconv.FormatUint(request.threadLimit, 10),
		"-A", request.release.architecture.path,
		"-C", request.release.configuration.path,
		"-I", initialPC,
		"-x", "-i", hex.EncodeToString(instruction.Bytes),
		"--timeout", strconv.FormatUint(request.timeLimit, 10),
	}
}
