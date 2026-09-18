// Program identity functions expose the exact generated and source artifacts.
// They calculate identities from bytes instead of caller-provided labels.
package isla

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/HyperMarble/hyperray/machine"
)

func threadEntryIdentity(values []ThreadEntry) string {
	parts := make([]string, len(values))
	for index := range values {
		parts[index] = fmt.Sprintf("0x%x", values[index].EntryAddress)
	}
	return strings.Join(parts, ",")
}

func copyThreadEntries(values []ThreadEntry) []ThreadEntry {
	result := make([]ThreadEntry, len(values))
	for index := range values {
		result[index] = values[index]
		result[index].InitialRegisters = append([]RegisterValue(nil), values[index].InitialRegisters...)
	}
	return result
}

func newProgram(image machine.Image, content []byte) Program {
	return Program{
		content:          append([]byte(nil), content...),
		digest:           contentDigest(content),
		imageDigest:      image.ArtifactSHA256,
		profile:          image.Profile,
		entryAddress:     image.EntryAddress,
		instructionCount: uint64(len(image.Instructions)),
		loadedByteCount:  uint64(len(image.LoadedBytes)),
		instructions:     copyProgramInstructions(image.Instructions),
	}
}

func copyProgramInstructions(values []machine.Instruction) []machine.Instruction {
	result := make([]machine.Instruction, len(values))
	for index := range values {
		result[index] = values[index]
		result[index].Bytes = append([]byte(nil), values[index].Bytes...)
	}
	return result
}

func contentDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
