// Certificate types carry claims checked against independent inventory data.
// They never act as their own source catalogs.
package coverage

type RootMapping struct {
	RootID                        string   `json:"root_id"`
	StateIDs                      []string `json:"state_ids"`
	ImpossiblePreconditionProofID string   `json:"impossible_precondition_proof_id,omitempty"`
	ProvenanceEdgeIDs             []string `json:"provenance_edge_ids"`
}

type MachineMapping struct {
	OperationID                       string `json:"operation_id"`
	CompilerOutputID                  string `json:"compiler_output_id"`
	InstructionID                     string `json:"instruction_id"`
	SemanticRuleID                    string `json:"semantic_rule_id"`
	TransitionID                      string `json:"transition_id"`
	OperationToCompilerOutputEdgeID   string `json:"operation_to_compiler_output_edge_id"`
	CompilerOutputToInstructionEdgeID string `json:"compiler_output_to_instruction_edge_id"`
	InstructionToSemanticRuleEdgeID   string `json:"instruction_to_semantic_rule_edge_id"`
	SemanticRuleToTransitionEdgeID    string `json:"semantic_rule_to_transition_edge_id"`
}

type SemanticMapping struct {
	OperationID                    string `json:"operation_id"`
	SemanticRuleID                 string `json:"semantic_rule_id"`
	TransitionID                   string `json:"transition_id"`
	OperationToSemanticRuleEdgeID  string `json:"operation_to_semantic_rule_edge_id"`
	SemanticRuleToTransitionEdgeID string `json:"semantic_rule_to_transition_edge_id"`
}

type EliminationMapping struct {
	RecordID                 string   `json:"record_id"`
	OperationID              string   `json:"operation_id"`
	ProofID                  string   `json:"proof_id"`
	EquivalentTransitionIDs  []string `json:"equivalent_transition_ids"`
	OperationToRecordEdgeID  string   `json:"operation_to_record_edge_id"`
	RecordToProofEdgeID      string   `json:"record_to_proof_edge_id"`
	ProofToTransitionEdgeIDs []string `json:"proof_to_transition_edge_ids"`
}

type Certificate struct {
	Roots       []RootMapping        `json:"roots"`
	Machine     []MachineMapping     `json:"machine"`
	Synthetic   []SemanticMapping    `json:"synthetic"`
	Environment []SemanticMapping    `json:"environment"`
	Eliminated  []EliminationMapping `json:"eliminated"`
}
