// Requirement types describe one explicit-graph proof question.
// They never select model states without validated coverage roots.
package proof

type RequirementKind string

const (
	RequirementSafety           RequirementKind = "safety"
	RequirementTotalTermination RequirementKind = "total_termination"
)

type Requirement struct {
	ID               string          `json:"id"`
	Kind             RequirementKind `json:"kind"`
	BadStateIDs      []string        `json:"bad_state_ids,omitempty"`
	BadTransitionIDs []string        `json:"bad_transition_ids,omitempty"`
	TerminalStateIDs []string        `json:"terminal_state_ids,omitempty"`
}

type Query struct {
	RootIDs     []string    `json:"root_ids"`
	Requirement Requirement `json:"requirement"`
}
