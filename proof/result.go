// Result types expose exact graph verdicts, closure data, and replayable traces.
// They never represent an engine error as a logic verdict.
package proof

import (
	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/model"
)

type Verdict string

const (
	VerdictProved    Verdict = "PROVED"
	VerdictDisproved Verdict = "DISPROVED"
)

type WitnessKind string

const (
	WitnessSafetyState         WitnessKind = "safety_state"
	WitnessSafetyTransition    WitnessKind = "safety_transition"
	WitnessTerminationDeadlock WitnessKind = "termination_deadlock"
	WitnessTerminationCycle    WitnessKind = "termination_cycle"
)

type Trace struct {
	States        []model.State `json:"states"`
	TransitionIDs []string      `json:"transition_ids"`
}

type Witness struct {
	Kind          WitnessKind `json:"kind"`
	RootID        string      `json:"root_id"`
	RequirementID string      `json:"requirement_id"`
	Cause         string      `json:"cause"`
	StateID       string      `json:"state_id"`
	TransitionID  string      `json:"transition_id,omitempty"`
	Path          Trace       `json:"path"`
	Cycle         *Trace      `json:"cycle,omitempty"`
}

type Result struct {
	Verdict                Verdict         `json:"verdict"`
	RequirementID          string          `json:"requirement_id"`
	Coverage               coverage.Report `json:"coverage"`
	ReachableStateIDs      []string        `json:"reachable_state_ids"`
	ReachableTransitionIDs []string        `json:"reachable_transition_ids"`
	Witness                *Witness        `json:"witness,omitempty"`
}
