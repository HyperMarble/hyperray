// Package model defines the v1 explicit finite-state graph.
// It never infers reachability or imposes a graph-size limit.
package model

// State is one exact graph state and its named witness values.
// Nil and empty Values maps both omit the values field from JSON.
type State struct {
	ID     string            `json:"id"`
	Values map[string]string `json:"values,omitempty"`
}

// Transition is one directed edge in the graph.
type Transition struct {
	ID          string `json:"id"`
	FromStateID string `json:"from_state_id"`
	ToStateID   string `json:"to_state_id"`
}

// Model is a finite explicit transition graph.
type Model struct {
	States      []State      `json:"states"`
	Transitions []Transition `json:"transitions"`
}
