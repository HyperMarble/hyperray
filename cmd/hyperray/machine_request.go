// The machine request describes one ARM64 binary and the property to prove.
// It must name every artifact explicitly and never infer a missing boundary.
package main

// machineRequest is the JSON input of the machine command.
type machineRequest struct {
	Binary    string          `json:"binary"`
	Tools     machineTools    `json:"tools"`
	Boundary  machineBoundary `json:"boundary"`
	Memory    machineMemory   `json:"memory"`
	Assertion string          `json:"negated_assertion"`
	Limits    machineLimits   `json:"limits"`
}

// machineTools names the measured native trio and its pinned inputs.
type machineTools struct {
	Solver         string `json:"solver"`
	Semantics      string `json:"semantics"`
	Footprints     string `json:"footprints"`
	Architecture   string `json:"architecture"`
	ArchitectureID string `json:"architecture_sha256"`
	Configuration  string `json:"configuration"`
	MemoryModel    string `json:"memory_model"`
	Manifest       string `json:"manifest"`
	ManifestID     string `json:"manifest_sha256"`
}

// machineBoundary states where execution starts and stops.
type machineBoundary struct {
	Name          string                `json:"name"`
	FunctionStart uint64                `json:"function_start"`
	FunctionEnd   uint64                `json:"function_end"`
	ReturnAddress uint64                `json:"return_address"`
	Registers     []machineRegisterPair `json:"post_reset_registers"`
	Observations  []machineObservation  `json:"memory_observations"`
}

type machineRegisterPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type machineObservation struct {
	Name    string `json:"name"`
	Address uint64 `json:"address"`
	Bytes   uint32 `json:"bytes"`
}
