// Footprint reports bind each instruction to model-produced semantic traces.
// They remain coverage evidence and do not claim circuit equivalence.
package isla

// InstructionTrace records Isla output for one executable instruction.
type InstructionTrace struct {
	Address             uint64                  `json:"address"`
	Encoding            string                  `json:"encoding"`
	TraceCount          uint64                  `json:"trace_count"`
	OutputDigest        string                  `json:"output_sha256"`
	ElapsedMilliseconds int64                   `json:"elapsed_milliseconds"`
	Diagnostics         string                  `json:"diagnostics,omitempty"`
	Dispositions        []DiagnosticDisposition `json:"diagnostic_dispositions,omitempty"`
}

// FootprintEvidence identifies every shared input and finite limit.
type FootprintEvidence struct {
	Tool                ToolIdentity `json:"tool"`
	ReleaseID           string       `json:"release_id"`
	ManifestDigest      string       `json:"manifest_sha256"`
	ArchitectureDigest  string       `json:"architecture_sha256"`
	ConfigurationDigest string       `json:"configuration_sha256"`
	ThreadLimit         uint64       `json:"thread_limit"`
	TimeLimitSeconds    uint64       `json:"time_limit_seconds"`
	MaximumOutputBytes  uint64       `json:"maximum_output_bytes"`
}

// FootprintReport contains one trace record for every accepted instruction.
type FootprintReport struct {
	Instructions []InstructionTrace `json:"instructions"`
	Evidence     FootprintEvidence  `json:"evidence"`
}
