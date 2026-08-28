// Package testir derives the verifier's exact pass predicate from complete,
// compiler-derived test dependency graphs. Optional bounded execution can
// cross-check that static authority, but can never author it.
package testir

import (
	"context"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

// TranslateFunc retranslates one derived candidate using the same pinned
// frontend/tool contract that translated the frozen reference.
type TranslateFunc func(context.Context, semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic)

// MaterializeFunc mechanically realizes a complete semantic behavior vector
// as exact source edits. The author does not provide mutation recipes.
type MaterializeFunc func(context.Context, semanticir.MaterializationRequest) (semanticir.EditPlan, []semanticir.Diagnostic)

// ArtifactBinding binds one frozen code artifact to its complete translation
// and the frontend operations needed to derive and revalidate candidates.
type ArtifactBinding struct {
	Frontend    semanticir.FrontendRequest `json:"frontend"`
	Model       semanticir.ArtifactModel   `json:"model"`
	Translate   TranslateFunc              `json:"-"`
	Materialize MaterializeFunc            `json:"-"`
}

// Request contains only frozen semantic/execution inputs. MaxVectors is an
// optional resource ceiling; reaching it blocks construction rather than
// allowing a sampled predicate to be used as proof.
type Request struct {
	Task      *semanticir.Task  `json:"task"`
	Artifacts []ArtifactBinding `json:"artifacts"`
	// TestModels are independently translated, complete models of every
	// frozen test artifact. Their conjunction authors TestsPass; executable
	// enumeration only confirms it.
	TestModels  []semanticir.ArtifactModel `json:"test_models"`
	Executor    executor.TaskEnvironment   `json:"executor"`
	MaxVectors  uint64                     `json:"max_vectors,omitempty"`
	Repetitions int                        `json:"repetitions,omitempty"`
}

// Status distinguishes an exact truth table from a fail-closed attempt.
type Status string

const (
	StatusComplete Status = "COMPLETE"
	StatusBlocked  Status = "PROOF BLOCKED"
	StatusNotRun   Status = "NOT RUN: RESOURCE BOUND"
)

type NotRunEvidence struct {
	Code          string `json:"code"`
	Detail        string `json:"detail"`
	TotalVectors  uint64 `json:"total_vectors"`
	VectorCeiling uint64 `json:"vector_ceiling"`
}

// Blocker is a stable, machine-readable reason that no proof may consume the
// attempted Test IR.
type Blocker struct {
	Stage    string `json:"stage"`
	Code     string `json:"code"`
	VectorID string `json:"vector_id,omitempty"`
	Detail   string `json:"detail"`
}

// RetranslationEvidence binds one derived candidate artifact to the exact
// bytes and complete frontend model that established its semantics.
type RetranslationEvidence struct {
	ArtifactID          string                       `json:"artifact_id"`
	CandidateSHA256     string                       `json:"candidate_sha256"`
	ModelSHA256         string                       `json:"model_sha256"`
	Model               semanticir.ArtifactModel     `json:"model"`
	Coverage            semanticir.TranslationStatus `json:"coverage"`
	CategoryProofSHA256 []string                     `json:"category_proof_sha256"`
}

// VectorEvidence is an immutable-by-digest record of one complete behavior
// vector. Baseline is true only for the exact frozen reference vector.
type VectorEvidence struct {
	ID                  string                       `json:"id"`
	Choices             []semanticir.BehaviorChoice  `json:"choices"`
	CandidateSHA256     string                       `json:"candidate_sha256"`
	Baseline            bool                         `json:"baseline"`
	Plans               []semanticir.EditPlan        `json:"plans"`
	Retranslations      []RetranslationEvidence      `json:"retranslations"`
	SemanticIsolation   SemanticIsolationEvidence    `json:"semantic_isolation"`
	Command             executor.CommandEvidence     `json:"command"`
	Commands            []executor.CommandEvidence   `json:"commands"`
	ExecutionIsolation  executor.IsolationEvidence   `json:"execution_isolation"`
	ExecutionIsolations []executor.IsolationEvidence `json:"execution_isolations"`
	PredictedTestsPass  bool                         `json:"predicted_tests_pass"`
	TestsPass           bool                         `json:"tests_pass"`
	EvidenceSHA256      string                       `json:"evidence_sha256"`
}

// Result contains an optional bounded executable cross-check of the exact
// static global TestPredicate. Predicate is unusable unless Status is COMPLETE
// and ValidateEvidence succeeds.
type Result struct {
	Status                Status                           `json:"status"`
	SemanticTask          semanticir.Task                  `json:"semantic_task"`
	SemanticTaskSHA256    string                           `json:"semantic_task_sha256"`
	Predicate             semanticir.TestPredicate         `json:"predicate"`
	TestModels            []semanticir.ArtifactModel       `json:"test_models"`
	TestModelDigests      []semanticir.ArtifactModelDigest `json:"test_model_digests"`
	StaticPredicateSHA256 string                           `json:"static_predicate_sha256"`
	PredictedVectorSHA256 string                           `json:"predicted_vector_sha256"`
	ObservedVectorSHA256  string                           `json:"observed_vector_sha256"`
	Harness               executor.TaskEnvironment         `json:"harness"`
	HarnessSHA256         string                           `json:"harness_sha256"`
	WorkspaceSHA256       string                           `json:"workspace_sha256"`
	TotalVectors          uint64                           `json:"total_vectors"`
	AcceptedVectors       uint64                           `json:"accepted_vectors"`
	Vectors               []VectorEvidence                 `json:"vectors"`
	Execution             executor.Report                  `json:"execution"`
	Executions            []executor.Report                `json:"executions"`
	Repetitions           int                              `json:"repetitions"`
	RunDigests            []string                         `json:"run_digests"`
	Blockers              []Blocker                        `json:"blockers,omitempty"`
	NotRun                *NotRunEvidence                  `json:"not_run,omitempty"`
	EvidenceSHA256        string                           `json:"evidence_sha256"`
}
