// Proposal results separate concrete differences from unvalidated UNSAT text.
// No result in this package is a proof or a semantic coverage certificate.
package circuit

// ProposalStatus identifies the meaning of an engine proposal.
type ProposalStatus string

const (
	// DifferenceFound identifies one concrete unequal step.
	DifferenceFound ProposalStatus = "difference_found"
	// UnvalidatedUnsat identifies UNSAT without an independent proof record.
	UnvalidatedUnsat ProposalStatus = "unvalidated_unsat"
)

// AssignmentRole identifies a current-state or nondeterministic input value.
type AssignmentRole string

const (
	StateAssignment AssignmentRole = "state"
	InputAssignment AssignmentRole = "input"
)

// OutputKind identifies the part of the step that differs.
type OutputKind string

const (
	NextStateOutput   OutputKind = "next_state"
	ObservationOutput OutputKind = "observation"
)

// Assignment is one concrete solver-selected bit-vector value.
type Assignment struct {
	Role  AssignmentRole `json:"role"`
	Name  string         `json:"name"`
	Value string         `json:"value"`
}

// Difference identifies the first output that differs at one assignment.
type Difference struct {
	Assignments    []Assignment `json:"assignments"`
	Kind           OutputKind   `json:"kind"`
	Name           string       `json:"name"`
	ReferenceValue string       `json:"reference_value"`
	CandidateValue string       `json:"candidate_value"`
}

// Proposal is either a concrete difference or explicit unvalidated UNSAT.
type Proposal struct {
	Status      ProposalStatus `json:"status"`
	Tool        ToolIdentity   `json:"tool"`
	MiterDigest string         `json:"miter_digest"`
	Difference  *Difference    `json:"difference,omitempty"`
}
