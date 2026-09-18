// Executable results retain the generated program, static traces, and verdict.
// Each part remains observable through the public result.
package isla

// ProgramEvidence binds generated solver input to one exact ELF image.
type ProgramEvidence struct {
	ProgramDigest      string              `json:"program_sha256"`
	ImageDigest        string              `json:"image_sha256"`
	Profile            string              `json:"profile"`
	FunctionStart      uint64              `json:"function_start"`
	FunctionEnd        uint64              `json:"function_end"`
	ReturnAddress      uint64              `json:"return_address"`
	EntryAddress       uint64              `json:"entry_address"`
	InstructionCount   uint64              `json:"instruction_count"`
	LoadedByteCount    uint64              `json:"loaded_byte_count"`
	MemoryProfile      MemoryProfile       `json:"memory_profile,omitempty"`
	MemoryIdentity     string              `json:"memory_identity,omitempty"`
	MemoryObservations []MemoryObservation `json:"memory_observations,omitempty"`
	ThreadCount        uint64              `json:"thread_count"`
	ThreadEntries      string              `json:"thread_entries"`
}

// ARM64CapabilityEvidence records trusted measurement metadata from the capability manifest.
// It is not a live post-reset observation. ProgramEvidence.MemoryIdentity remains the per-query caller-memory identity.
type ARM64CapabilityEvidence struct {
	CapabilityID              string `json:"capability_id"`
	ExecutionProfile          string `json:"execution_profile"`
	MemoryProfile             string `json:"memory_profile"`
	MemoryConfigurationDigest string `json:"memory_config_sha256"`
	EffectiveStateDigest      string `json:"effective_state_sha256"`
}

// ExecutableLimits bounds static instruction tracing for one program.
type ExecutableLimits struct {
	ThreadLimit        uint64 `json:"thread_limit"`
	TimeLimitSeconds   uint64 `json:"time_limit_seconds"`
	MaximumOutputBytes uint64 `json:"maximum_output_bytes"`
}

// ExecutableResult contains one fully joined machine verification result.
type ExecutableResult struct {
	Program        ProgramEvidence          `json:"program"`
	Capability     *ARM64CapabilityEvidence `json:"capability,omitempty"`
	Footprints     FootprintReport          `json:"footprints"`
	StaticCoverage FootprintCoverage        `json:"static_coverage"`
	Execution      ExecutionInventory       `json:"execution"`
	Verification   VerificationResult       `json:"verification"`
}
