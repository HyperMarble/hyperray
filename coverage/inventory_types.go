// Inventory types declare independent compiler, semantic, and provenance data.
// They never obtain identities from certificate rows.
package coverage

type CompilerOutput struct {
	ID       string            `json:"id"`
	Artifact ArtifactReference `json:"artifact"`
}

type ImageInstruction struct {
	ID       string            `json:"id"`
	Artifact ArtifactReference `json:"artifact"`
}

type SemanticRule struct {
	ID       string            `json:"id"`
	Artifact ArtifactReference `json:"artifact"`
}

type ImpossiblePreconditionProof struct {
	ID       string            `json:"id"`
	Artifact ArtifactReference `json:"artifact"`
}

type EliminationProof struct {
	ID       string            `json:"id"`
	Artifact ArtifactReference `json:"artifact"`
}

type RootEntry struct {
	RootID                        string   `json:"root_id"`
	StateIDs                      []string `json:"state_ids"`
	ImpossiblePreconditionProofID string   `json:"impossible_precondition_proof_id,omitempty"`
	ProvenanceEdgeIDs             []string `json:"provenance_edge_ids"`
}

type EliminationRecord struct {
	ID                       string   `json:"id"`
	OperationID              string   `json:"operation_id"`
	ProofID                  string   `json:"proof_id"`
	EquivalentTransitionIDs  []string `json:"equivalent_transition_ids"`
	OperationToRecordEdgeID  string   `json:"operation_to_record_edge_id"`
	RecordToProofEdgeID      string   `json:"record_to_proof_edge_id"`
	ProofToTransitionEdgeIDs []string `json:"proof_to_transition_edge_ids"`
}
