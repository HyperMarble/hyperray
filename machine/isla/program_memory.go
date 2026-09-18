// Memory profiles are explicit caller contracts, not inferred optimizations.
// A sequential profile must never replace concurrent execution silently.
package isla

type MemoryProfile string

const (
	AxiomaticMemory          MemoryProfile = ""
	SequentialMemory         MemoryProfile = "sequential"
	ARM64FetchOnlyV1         MemoryProfile = "arm64-fetch-only-v1"
	ARM64NormalS1Fixed4KSIMD MemoryProfile = "arm64-normal-s1-fixed-4k-simd-v1"
)

func validateMemoryProfile(profile MemoryProfile) error {
	switch profile {
	case AxiomaticMemory, SequentialMemory:
		return nil
	default:
		return engineError(InvalidInput, "memory profile", string(profile))
	}
}
