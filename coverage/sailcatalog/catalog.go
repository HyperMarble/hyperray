// Package sailcatalog validates exact typed-Sail instruction-family coverage.
// It keeps declarations separate from decoder and execution ownership.
package sailcatalog

type Catalog struct {
	Declarations []string `json:"instruction_declarations"`
	Decoded      []string `json:"decode_clauses"`
	Executed     []string `json:"execute_clauses"`
}

type Report struct {
	Complete bool `json:"complete"`
	Families int  `json:"instruction_families"`
}
