// Package jibcatalog reconciles JIB origins with circuit-lowering evidence.
// It never treats equal counts as semantic coverage.
package jibcatalog

type Disposition string

const (
	DispositionEmitted Disposition = "emitted"
	DispositionErased  Disposition = "erased_with_proof"
)

type Origin struct {
	ID       string `json:"origin_id"`
	Category string `json:"category"`
	Kind     string `json:"kind"`
}

type Translation struct {
	OriginID          string      `json:"origin_id"`
	RuleID            string      `json:"rule_id"`
	ProofObligationID string      `json:"proof_obligation_id"`
	Disposition       Disposition `json:"disposition"`
	RegionIDs         []string    `json:"region_ids"`
}

type Region struct {
	ID string `json:"region_id"`
}

type Catalog struct {
	Origins      []Origin      `json:"origins"`
	Translations []Translation `json:"translations"`
	Regions      []Region      `json:"regions"`
}

type Report struct {
	Complete     bool `json:"complete"`
	Origins      int  `json:"jib_origins"`
	Translations int  `json:"translations"`
	Regions      int  `json:"circuit_regions"`
}
