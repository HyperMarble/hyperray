// Event catalogs expose every generic Isla event before circuit translation.
// They must not classify an instruction or omit an observed event.
package isla

// TraceEvent identifies one event in one retained instruction trace.
type TraceEvent struct {
	Identifier  string `json:"identifier"`
	Address     uint64 `json:"address"`
	TraceIndex  uint64 `json:"trace_index"`
	EventIndex  uint64 `json:"event_index"`
	Kind        string `json:"kind"`
	TraceDigest string `json:"trace_sha256"`
}

// TraceEventCatalog contains the complete ordered event inventory.
type TraceEventCatalog struct {
	Complete         bool         `json:"complete"`
	InstructionCount uint64       `json:"instruction_count"`
	TraceCount       uint64       `json:"trace_count"`
	EventCount       uint64       `json:"event_count"`
	Events           []TraceEvent `json:"events"`
}
