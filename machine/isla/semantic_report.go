// Semantic reports retain the complete model output and measured coverage sets.
// They do not contain a solver verdict.
package isla

// SemanticEvidence binds semantic output to its tool, inputs, and limits.
type SemanticEvidence struct {
	Tool                ToolIdentity `json:"tool"`
	ArchitectureDigest  string       `json:"architecture_sha256"`
	ConfigurationDigest string       `json:"configuration_sha256"`
	ProgramDigest       string       `json:"program_sha256"`
	OutputDigest        string       `json:"output_sha256"`
	ThreadLimit         uint64       `json:"thread_limit"`
	PCVisitLimit        uint64       `json:"pc_visit_limit"`
	TimeLimitSeconds    uint64       `json:"time_limit_seconds"`
	MemoryLimitMB       uint64       `json:"memory_limit_mb"`
	MaximumOutputBytes  uint64       `json:"maximum_output_bytes"`
	ElapsedMilliseconds int64        `json:"elapsed_milliseconds"`
}

// SemanticReport records every accepted program semantic tree.
type SemanticReport struct {
	Complete              bool                    `json:"complete"`
	ThreadCount           uint64                  `json:"thread_count"`
	TraceCount            uint64                  `json:"trace_count"`
	InstructionEventCount uint64                  `json:"instruction_event_count"`
	InstructionEncodings  []string                `json:"instruction_encodings"`
	Instructions          []SemanticInstruction   `json:"instructions"`
	Threads               []SemanticThread        `json:"threads"`
	FootprintEncodings    []string                `json:"footprint_encodings"`
	FinalAssertion        string                  `json:"final_assertion"`
	TraceOutput           string                  `json:"trace_output"`
	Diagnostics           string                  `json:"diagnostics,omitempty"`
	Dispositions          []DiagnosticDisposition `json:"diagnostic_dispositions,omitempty"`
	Evidence              SemanticEvidence        `json:"evidence"`
}

// SemanticThread retains the instructions and entry observed for one report thread.
type SemanticThread struct {
	ID                    uint64                `json:"id"`
	EntryAddress          uint64                `json:"entry_address"`
	EntryAddresses        []uint64              `json:"entry_addresses"`
	InstructionEventCount uint64                `json:"instruction_event_count"`
	Instructions          []SemanticInstruction `json:"instructions"`
}

type semanticSummary struct {
	threadCount           uint64
	traceCount            uint64
	instructionEventCount uint64
	instructionEncodings  []string
	instructions          []SemanticInstruction
	threads               []SemanticThread
	footprintEncodings    []string
	finalAssertion        string
}
