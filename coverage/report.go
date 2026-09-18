// Report exposes exact distinct counts for every reconciled catalog.
// It never reports a partial certificate as complete.
package coverage

type Report struct {
	Complete                     bool `json:"complete"`
	RootedFunctions              int  `json:"rooted_functions"`
	TotalFunctions               int  `json:"total_functions"`
	MappedRoots                  int  `json:"mapped_roots"`
	ProvedImpossibleRoots        int  `json:"proved_impossible_roots"`
	CoveredRoots                 int  `json:"covered_roots"`
	TotalRoots                   int  `json:"total_roots"`
	MappedOperations             int  `json:"mapped_operations"`
	EliminatedOperations         int  `json:"eliminated_operations"`
	CoveredOperations            int  `json:"covered_operations"`
	TotalOperations              int  `json:"total_operations"`
	ReconciledArtifacts          int  `json:"reconciled_artifacts"`
	TotalArtifacts               int  `json:"total_artifacts"`
	ReconciledCompilerOutputs    int  `json:"reconciled_compiler_outputs"`
	TotalCompilerOutputs         int  `json:"total_compiler_outputs"`
	ReconciledImageInstructions  int  `json:"reconciled_image_instructions"`
	TotalImageInstructions       int  `json:"total_image_instructions"`
	ReconciledSemanticRules      int  `json:"reconciled_semantic_rules"`
	TotalSemanticRules           int  `json:"total_semantic_rules"`
	ReconciledProvenanceEdges    int  `json:"reconciled_provenance_edges"`
	TotalProvenanceEdges         int  `json:"total_provenance_edges"`
	ReconciledImpossibleProofs   int  `json:"reconciled_impossible_proofs"`
	TotalImpossibleProofs        int  `json:"total_impossible_proofs"`
	ReconciledEliminationProofs  int  `json:"reconciled_elimination_proofs"`
	TotalEliminationProofs       int  `json:"total_elimination_proofs"`
	ReconciledEliminationRecords int  `json:"reconciled_elimination_records"`
	TotalEliminationRecords      int  `json:"total_elimination_records"`
	MappedTransitions            int  `json:"mapped_transitions"`
	TotalTransitions             int  `json:"total_transitions"`
}
