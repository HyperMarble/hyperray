// Provenance types give every evidence edge typed logical endpoints.
// They never use an artifact name as a semantic endpoint.
package coverage

type ProvenanceNodeKind string

const (
	NodeOperation                   ProvenanceNodeKind = "operation"
	NodeCompilerOutput              ProvenanceNodeKind = "compiler_output"
	NodeImageInstruction            ProvenanceNodeKind = "image_instruction"
	NodeSemanticRule                ProvenanceNodeKind = "semantic_rule"
	NodeModelTransition             ProvenanceNodeKind = "model_transition"
	NodeRoot                        ProvenanceNodeKind = "root"
	NodeModelState                  ProvenanceNodeKind = "model_state"
	NodeEliminationRecord           ProvenanceNodeKind = "elimination_record"
	NodeEliminationProof            ProvenanceNodeKind = "elimination_proof"
	NodeImpossiblePreconditionProof ProvenanceNodeKind = "impossible_precondition_proof"
)

type ProvenanceNode struct {
	Kind ProvenanceNodeKind `json:"kind"`
	ID   string             `json:"id"`
}

type ProvenanceEdge struct {
	ID       string            `json:"id"`
	From     ProvenanceNode    `json:"from"`
	To       ProvenanceNode    `json:"to"`
	Artifact ArtifactReference `json:"artifact"`
}
