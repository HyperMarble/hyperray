//go:build preparation_integration && darwin

// Integration inputs mirror the public Rust preparation protocol.
// This test schema must not supply subject behavior to production code.
package execution_test

import "github.com/HyperMarble/hyperray/execution"

type preparationFunction struct {
	Source   string `json:"source"`
	Function string `json:"function"`
}

type preparationLimits struct {
	Minimum           uint64 `json:"minimum"`
	Maximum           uint64 `json:"maximum"`
	SearchDepth       uint32 `json:"search_depth"`
	HashBits          uint8  `json:"hash_bits"`
	StateMemoryMiB    uint32 `json:"state_memory_mib"`
	TimeoutMS         uint64 `json:"timeout_ms"`
	OutputLimitBytes  uint64 `json:"output_limit_bytes"`
	MemoryBudgetBytes uint64 `json:"memory_budget_bytes"`
}

type preparationRequest struct {
	Subject      preparationFunction      `json:"subject"`
	Requirement  preparationFunction      `json:"requirement"`
	Tools        map[string]string        `json:"tools"`
	Directory    string                   `json:"directory"`
	Optimization uint8                    `json:"optimization"`
	Limits       preparationLimits        `json:"limits"`
	Cargo        *cargoPreparationOptions `json:"cargo,omitempty"`
}

type preparedResult struct {
	Execution execution.Request `json:"execution"`
	Replay    string            `json:"replay_executable"`
	InputBits uint32            `json:"input_bits"`
	Minimum   uint64            `json:"minimum"`
	Maximum   uint64            `json:"maximum"`
	Tools     []struct {
		Tool    string
		Version string
	} `json:"tools"`
}
