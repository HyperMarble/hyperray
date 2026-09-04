// Footprint coverage reports exact instruction-inventory reconciliation.
package isla

// FootprintCoverage is complete only after both inventories match exactly.
type FootprintCoverage struct {
	Complete            bool   `json:"complete"`
	CoveredInstructions uint64 `json:"covered_instructions"`
	TotalInstructions   uint64 `json:"total_instructions"`
}
