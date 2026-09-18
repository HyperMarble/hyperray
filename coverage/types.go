// Package coverage checks complete semantic evidence against an explicit model.
// It never uses reachability to reduce the coverage requirement.
package coverage

// OperationKind identifies the source of an inventoried operation.
type OperationKind string

const (
	OperationCompiler    OperationKind = "compiler"
	OperationSynthetic   OperationKind = "synthetic"
	OperationEnvironment OperationKind = "environment"
)

// OperationDisposition states how an operation has coverage.
type OperationDisposition string

const (
	DispositionMapped     OperationDisposition = "mapped"
	DispositionEliminated OperationDisposition = "eliminated_with_proof"
)

type Function struct {
	ID string `json:"id"`
}

type Root struct {
	ID         string `json:"id"`
	FunctionID string `json:"function_id"`
}

type Operation struct {
	ID          string               `json:"id"`
	FunctionID  string               `json:"function_id,omitempty"`
	Kind        OperationKind        `json:"kind"`
	Location    string               `json:"location"`
	Disposition OperationDisposition `json:"disposition"`
}

// ArtifactReference binds an artifact name to its exact content digest.
type ArtifactReference struct {
	ArtifactID string `json:"artifact_id"`
	SHA256     string `json:"sha256"`
}

type Artifact struct {
	ID      string `json:"id"`
	SHA256  string `json:"sha256"`
	Content []byte `json:"content"`
}
