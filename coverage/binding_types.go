// Binding types declare the expected ownership of every model transition.
// They never let certificate rows define that ownership.
package coverage

type MachineBinding struct {
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

type SemanticBinding struct {
	OperationID                    string        `json:"operation_id"`
	Kind                           OperationKind `json:"kind"`
	SemanticRuleID                 string        `json:"semantic_rule_id"`
	TransitionID                   string        `json:"transition_id"`
	OperationToSemanticRuleEdgeID  string        `json:"operation_to_semantic_rule_edge_id"`
	SemanticRuleToTransitionEdgeID string        `json:"semantic_rule_to_transition_edge_id"`
}

type CompilerInventory struct {
	Functions                    []Function                    `json:"functions"`
	Roots                        []Root                        `json:"roots"`
	Operations                   []Operation                   `json:"operations"`
	Artifacts                    []Artifact                    `json:"artifacts"`
	CompilerOutputs              []CompilerOutput              `json:"compiler_outputs"`
	ImageInstructions            []ImageInstruction            `json:"image_instructions"`
	SemanticRules                []SemanticRule                `json:"semantic_rules"`
	ProvenanceEdges              []ProvenanceEdge              `json:"provenance_edges"`
	RootEntries                  []RootEntry                   `json:"root_entries"`
	MachineBindings              []MachineBinding              `json:"machine_bindings"`
	SemanticBindings             []SemanticBinding             `json:"semantic_bindings"`
	ImpossiblePreconditionProofs []ImpossiblePreconditionProof `json:"impossible_precondition_proofs"`
	EliminationProofs            []EliminationProof            `json:"elimination_proofs"`
	EliminationRecords           []EliminationRecord           `json:"elimination_records"`
}
