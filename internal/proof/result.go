// Package proof proves the three finite implications and independently checks
// reference acceptance required by Hyperray.
package proof

import (
	"context"
	"fmt"

	"github.com/HyperMarble/hyperray/internal/executor"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// Verdict is the only public proof verdict vocabulary.
type Verdict string

const (
	VerdictVerified     Verdict = "VERIFIED"
	VerdictNotVerified  Verdict = "NOT VERIFIED"
	VerdictProofBlocked Verdict = "PROOF BLOCKED"
)

// Blocker explains why an exact proof could not be attempted.
type Blocker struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Provenance *semanticir.Provenance `json:"provenance,omitempty"`
}

func (b Blocker) Error() string {
	if b.Code == "" {
		return b.Message
	}
	return fmt.Sprintf("%s: %s", b.Code, b.Message)
}

// ObligationResult is one independently checkable finite set inclusion.
type ObligationResult struct {
	Obligation     semanticir.ProofObligation `json:"obligation"`
	Verdict        Verdict                    `json:"verdict"`
	Witness        *semanticir.Counterexample `json:"witness,omitempty"`
	Blockers       []Blocker                  `json:"blockers,omitempty"`
	Method         string                     `json:"method"`
	Exhaustive     bool                       `json:"exhaustive"`
	ReachableCases uint64                     `json:"reachable_cases"`
	OutcomeChecks  uint64                     `json:"outcome_checks"`
}

// Transcript accounts for the complete finite universe used by a Result.
type Transcript struct {
	Method                   string                                   `json:"method"`
	Complete                 bool                                     `json:"complete"`
	DomainAssignments        uint64                                   `json:"domain_assignments"`
	ExcludedAssignments      uint64                                   `json:"excluded_assignments"`
	ReachableAssignments     uint64                                   `json:"reachable_assignments"`
	ReachableCases           uint64                                   `json:"reachable_cases"`
	OutcomeUniverse          uint64                                   `json:"outcome_universe"`
	SpecIRDigest             string                                   `json:"spec_ir_digest"`
	ReferenceIRDigest        string                                   `json:"reference_ir_digest"`
	TestIRDigest             string                                   `json:"test_ir_digest"`
	EnvironmentIRDigest      string                                   `json:"environment_ir_digest"`
	CompilerEvidence         []semanticir.CompilerEvidence            `json:"compiler_evidence"`
	CompilerEvidenceSHA256   string                                   `json:"compiler_evidence_sha256"`
	DerivationReplays        []DerivationReplayBinding                `json:"derivation_replays"`
	DerivationReplaysSHA256  string                                   `json:"derivation_replays_sha256"`
	ScopeClosures            []semanticir.ScopeClosureEvidence        `json:"scope_closures"`
	ScopeClosuresSHA256      string                                   `json:"scope_closures_sha256"`
	ExhaustiveEvidence       []semanticir.ExhaustiveExecutionEvidence `json:"exhaustive_evidence"`
	ExhaustiveEvidenceSHA256 string                                   `json:"exhaustive_evidence_sha256"`
	TestSuite                *semanticir.TestSuiteModel               `json:"test_suite"`
	TestSuiteSHA256          string                                   `json:"test_suite_sha256"`
	Solver                   *SolverTranscript                        `json:"solver,omitempty"`
}

// DerivationReplayBinding is the deterministic, certificate-safe projection
// of an independently replayed CompilerSemanticGraph. Executor timestamps,
// durations, and disposable paths are intentionally excluded; every frozen
// plan/graph/source/tool/output identity and both-run result remains bound.
type DerivationReplayBinding struct {
	PlanSHA256              string                     `json:"plan_sha256"`
	GraphSHA256             string                     `json:"graph_sha256"`
	WorkspaceSHA256         string                     `json:"workspace_sha256"`
	SourceBindings          []executor.BindingEvidence `json:"source_bindings"`
	ToolBindings            []executor.BindingEvidence `json:"tool_bindings"`
	IRSHA256                string                     `json:"ir_sha256"`
	DecoderOutputSHA256     string                     `json:"decoder_output_sha256"`
	Repetitions             int                        `json:"repetitions"`
	Deterministic           bool                       `json:"deterministic"`
	OriginalWorkspaceIntact bool                       `json:"original_workspace_intact"`
}

// SolverTranscript binds every non-enumerated proof query to a frozen solver.
type SolverTranscript struct {
	Name              string                           `json:"name"`
	Version           string                           `json:"version"`
	Digest            string                           `json:"digest"`
	Tool              semanticir.ToolRef               `json:"tool"`
	Argv              []string                         `json:"argv"`
	WorkingDirectory  string                           `json:"working_directory"`
	Environment       []semanticir.EnvironmentVariable `json:"environment"`
	EnvironmentDigest string                           `json:"environment_digest"`
	ClearEnvironment  bool                             `json:"clear_environment"`
	KillProcessGroup  bool                             `json:"kill_process_group"`
	TimeoutMillis     int64                            `json:"timeout_millis"`
	Queries           []SolverQuery                    `json:"queries"`
}

// SolverQuery records the canonical formula digest and exact solver answer.
type SolverQuery struct {
	Obligation        semanticir.ProofObligation `json:"obligation"`
	SMTLIB            string                     `json:"smtlib"`
	SMTLIBSHA256      string                     `json:"smtlib_sha256"`
	Output            string                     `json:"output"`
	OutputSHA256      string                     `json:"output_sha256"`
	ModelSMTLIB       string                     `json:"model_smtlib,omitempty"`
	ModelSMTLIBSHA256 string                     `json:"model_smtlib_sha256,omitempty"`
	ModelOutput       string                     `json:"model_output,omitempty"`
	ModelOutputSHA256 string                     `json:"model_output_sha256,omitempty"`
	Result            string                     `json:"result"`
}

// Result records all four obligations independently. Counterexamples repeats
// the non-nil obligation witnesses in obligation order for pipeline consumers.
type Result struct {
	Verdict             Verdict                     `json:"verdict"`
	Reference           ObligationResult            `json:"reference"`
	FalsePositive       ObligationResult            `json:"false_positive"`
	Fairness            ObligationResult            `json:"fairness"`
	ReferenceAcceptance ObligationResult            `json:"reference_acceptance"`
	Counterexamples     []semanticir.Counterexample `json:"counterexamples,omitempty"`
	Blockers            []Blocker                   `json:"blockers,omitempty"`
	Transcript          Transcript                  `json:"transcript"`
}

// ValidateResult independently recomputes the proof against the frozen task
// and requires canonical byte equality with the supplied result. This checks
// aggregate verdicts, all obligation witnesses/blockers, method closure,
// compiler/test-suite bindings, solver ToolRefs, queries, raw outputs, and
// transcript digests without duplicating those invariants in certificates.
func ValidateResult(task *semanticir.Task, result Result) error {
	want := Verify(context.Background(), task)
	wantDigest, err := semanticir.Digest(want)
	if err != nil {
		return fmt.Errorf("digest recomputed proof result: %w", err)
	}
	gotDigest, err := semanticir.Digest(result)
	if err != nil {
		return fmt.Errorf("digest supplied proof result: %w", err)
	}
	if gotDigest != wantDigest {
		return fmt.Errorf("proof result is stale, tampered, or internally inconsistent: got %s, want %s", gotDigest, wantDigest)
	}
	return nil
}
