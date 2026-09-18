// ARM64 memory identity binds the complete typed contract to Program evidence.
// It must change when profile, arena, mappings, or caller bytes change.
package isla

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func memoryProfileOf(input *ARM64MemoryInput) MemoryProfile {
	if input == nil {
		return ARM64FetchOnlyV1
	}
	return input.Profile
}

func memoryIdentityOf(input *ARM64MemoryInput) string {
	if input == nil {
		return ""
	}
	text := fmt.Sprintf("%q|%d|%d", input.Profile, input.Table.Base, input.Table.CapacityPages)
	for _, mapping := range input.Mappings {
		text += fmt.Sprintf("|%d|%d|%d|%q", mapping.VA, mapping.PA, mapping.Length, mapping.Permission)
	}
	for _, backing := range input.Backing {
		text += fmt.Sprintf("|%d|%q|%x", backing.Address, backing.Permission, backing.Bytes)
	}
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}
