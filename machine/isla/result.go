// Proposal values separate counterexamples from the absence of one.
// They must not claim semantic coverage or a final proof.
package isla

// ProposalStatus identifies the solver result for a negated property.
type ProposalStatus string

const (
	CounterexampleFound   ProposalStatus = "counterexample_found"
	NoCounterexampleFound ProposalStatus = "no_counterexample_found"
)

// Evidence binds a proposal to its engine, inputs, limits, and raw result.
type Evidence struct {
	Tool                ToolIdentity `json:"tool"`
	ArchitectureDigest  string       `json:"architecture_sha256"`
	ConfigurationDigest string       `json:"configuration_sha256"`
	MemoryModelDigest   string       `json:"memory_model_sha256"`
	ProgramDigest       string       `json:"program_sha256"`
	OutputDigest        string       `json:"output_sha256"`
	PCVisitLimit        uint64       `json:"pc_visit_limit"`
	TimeLimitSeconds    uint64       `json:"time_limit_seconds"`
	ElapsedMilliseconds int64        `json:"elapsed_milliseconds"`
	Diagnostics         string       `json:"diagnostics,omitempty"`
}

// Proposal is an Isla result that still requires Hyperray coverage evidence.
type Proposal struct {
	Status              ProposalStatus `json:"status"`
	QueryName           string         `json:"query_name"`
	CandidateCount      uint64         `json:"candidate_count"`
	CounterexampleCount uint64         `json:"counterexample_count"`
	CounterexampleState string         `json:"counterexample_state,omitempty"`
	Evidence            Evidence       `json:"evidence"`
}
