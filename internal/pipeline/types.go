// Package pipeline is Hyperray's fail-closed production verification path.
//
// A successful result is deliberately difficult to construct: Run executes
// every stage against one frozen task and returns VERIFIED only after all
// translations and proof obligations are complete, executable confirmation
// has run, the frozen inputs are still current, and a tamper-evident
// certificate has been issued and verified.
package pipeline

import (
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// Verdict is the only externally visible result vocabulary. Keep these
// strings synchronized with certificate.Verdict; the pipeline converts to
// the certificate type at the issuance boundary instead of maintaining a
// second semantic verdict.
type Verdict string

const (
	Verified     Verdict = "VERIFIED"
	NotVerified  Verdict = "NOT VERIFIED"
	ProofBlocked Verdict = "PROOF BLOCKED"
)

// StageName identifies the mandatory stages in dependency order.
type StageName string

const (
	StageFreeze              StageName = "freeze"
	StageCompileSpec         StageName = "compile-spec"
	StageDiagnostics         StageName = "diagnostics"
	StageTranslateReference  StageName = "translate-reference"
	StageTranslateTests      StageName = "translate-tests"
	StageCompileTestIR       StageName = "compile-test-ir"
	StageProofReference      StageName = "proof-reference-within-spec"
	StageProofTestsSound     StageName = "proof-tests-pass-within-spec"
	StageProofTestsComplete  StageName = "proof-spec-within-tests-pass"
	StageReferenceAcceptance StageName = "reference-accepted-by-tests"
	StageConfirm             StageName = "confirm-counterexamples"
	StageCertificate         StageName = "certificate"
)

// StageStatus has no skipped/success-by-default value. A stage either
// completed, established a counterexample, or blocked proof.
type StageStatus string

const (
	StageComplete StageStatus = "complete"
	StageRefuted  StageStatus = "refuted"
	StageBlocked  StageStatus = "blocked"
)

// Stage records an auditable pipeline transition. Evidence contains digests
// and stable IDs only; prose is explanatory and never establishes proof.
type Stage struct {
	Name       StageName     `json:"name"`
	Status     StageStatus   `json:"status"`
	Evidence   []string      `json:"evidence,omitempty"`
	Diagnostic []string      `json:"diagnostic,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// Translation documents the strict hyperray.toml lowering declaration. Proof
// domains, constraints, operations, and outcomes always come from the compiled
// frozen spec. The config deliberately contains no duplicate semantic truth.
type Translation struct {
	ArtifactID          string                  `toml:"artifact_id"`
	WorkspacePath       string                  `toml:"workspace_path"`
	CompilationDatabase string                  `toml:"compilation_database"`
	ToolName            string                  `toml:"tool_name"`
	ProverToolName      string                  `toml:"prover_tool_name"`
	Language            semanticir.Language     `toml:"language"`
	Kind                semanticir.ArtifactKind `toml:"kind"`
	TestRole            string                  `toml:"test_role"`
	EntryPoints         []string                `toml:"entry_points"`
	ObservedOperations  []string                `toml:"observed_operations"`
	Options             map[string]string       `toml:"options"`
}

// Request selects a task and, optionally, non-default config/certificate
// locations. Paths are resolved relative to Root.
type Request struct {
	Root            string
	ConfigPath      string
	CertificatePath string
}

// Result is returned for every completed attempt, including blocked ones.
// CertificatePath is empty when the pipeline could not safely issue one.
type Result struct {
	Verdict         Verdict  `json:"verdict"`
	Stages          []Stage  `json:"stages"`
	Blockers        []string `json:"blockers,omitempty"`
	Counterexamples []string `json:"counterexamples,omitempty"`
	ManifestDigest  string   `json:"manifest_digest,omitempty"`
	IRDigest        string   `json:"ir_digest,omitempty"`
	CertificatePath string   `json:"certificate_path,omitempty"`
}

// Successful reports the only process-success verdict.
func (r Result) Successful() bool { return r.Verdict == Verified }
