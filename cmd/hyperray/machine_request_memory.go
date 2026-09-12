// These types declare what memory a legal caller supplies.
// They must state permissions explicitly and never imply a default region.
package main

// machineMemory declares what memory a legal caller supplies.
type machineMemory struct {
	TableBase  uint64           `json:"table_base"`
	TablePages uint32           `json:"table_capacity_pages"`
	Mappings   []machineMapping `json:"mappings"`
	Backing    []machineBacking `json:"backing"`
}

type machineMapping struct {
	VA         uint64 `json:"va"`
	PA         uint64 `json:"pa"`
	Length     uint64 `json:"length"`
	Permission string `json:"permission"`
}

type machineBacking struct {
	Address    uint64 `json:"address"`
	Permission string `json:"permission"`
	Bytes      string `json:"bytes"`
}

// machineLimits bounds the run so a slow query fails instead of hanging.
type machineLimits struct {
	MaximumLoadedBytes uint64 `json:"maximum_loaded_bytes"`
	MaximumOutputBytes uint64 `json:"maximum_output_bytes"`
	TimeLimitSeconds   uint64 `json:"time_limit_seconds"`
	PCVisitLimit       uint64 `json:"pc_visit_limit"`
}
