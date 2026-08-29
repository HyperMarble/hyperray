// Package semanticir defines the finite, language-neutral model shared by
// Hyperray's specification compiler, language frontends, and proof engine.
//
// The proof-critical IR is intentionally a small closed behavioral
// projection: finite labels, compiler-grounded partitions, outcomes/effects,
// code behavior, and a global test predicate. Optional high-level nodes are
// audit detail only; source-language proof comes from CompilerEvidence.
package semanticir

// Language is a source language supported by Hyperray's v0.1 contract.
type Language string

const (
	LanguageNone   Language = ""
	LanguagePython Language = "python"
	LanguageRust   Language = "rust"
	LanguageCPP    Language = "cpp"
)

// ArtifactKind identifies the independent role of a frozen input artifact.
type ArtifactKind string

const (
	ArtifactInstruction ArtifactKind = "instruction"
	ArtifactSpec        ArtifactKind = "spec"
	// ArtifactSource is a role-neutral frozen source file. FrontendRequest.Kind
	// and SourceRanges select an independent code or test translation from the
	// same immutable bytes without duplicating/relabeling the artifact.
	ArtifactSource              ArtifactKind = "source"
	ArtifactCode                ArtifactKind = "code"
	ArtifactTests               ArtifactKind = "tests"
	ArtifactEnvironment         ArtifactKind = "environment"
	ArtifactConfiguration       ArtifactKind = "configuration"
	ArtifactSpecAuthoringRecord ArtifactKind = "spec-authoring-record"
	ArtifactSpecLedger          ArtifactKind = "spec-ledger"
)

// ArtifactRef identifies immutable bytes. Digest is always a lowercase
// sha256:<64 hexadecimal digits> value when the reference is used as evidence.
type ArtifactRef struct {
	ID     string       `json:"id"`
	Kind   ArtifactKind `json:"kind"`
	Path   string       `json:"path"`
	Digest string       `json:"digest"`
}

// ToolRef binds a translator/solver/executor identity to immutable executable
// bytes and an exact reported version.
type ToolRef struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Version string `json:"version"`
}

// WorkspaceState names one of the frozen task workspaces.
type WorkspaceState string

const (
	WorkspaceBaseOldTests     WorkspaceState = "base-old-tests"
	WorkspaceBaseNewTests     WorkspaceState = "base-new-tests"
	WorkspaceSolutionNewTests WorkspaceState = "solution-new-tests"
)

// WorkspaceEntry binds a workspace-relative path to frozen artifact bytes.
type WorkspaceEntry struct {
	Path       string      `json:"path"`
	Artifact   ArtifactRef `json:"artifact"`
	Provenance Provenance  `json:"provenance"`
}

type ChangedSourceRange struct {
	ArtifactID  string     `json:"artifact_id"`
	Path        string     `json:"path"`
	StartLine   int        `json:"start_line"`
	EndLine     int        `json:"end_line"`
	SliceDigest string     `json:"slice_digest"`
	Provenance  Provenance `json:"provenance"`
}

// SourceRoleRange selects one exact byte interval from a role-neutral source
// artifact. EndByte is exclusive. SliceDigest hashes source[start:end].
// Ranges are ordered, non-overlapping, and copied byte-for-byte into the
// resulting ArtifactModel.
type SourceRoleRange struct {
	ArtifactID  string     `json:"artifact_id"`
	Path        string     `json:"path"`
	StartByte   int        `json:"start_byte"`
	EndByte     int        `json:"end_byte"`
	SliceDigest string     `json:"slice_digest"`
	Provenance  Provenance `json:"provenance"`
}

// WorkspaceRef gives a frontend complete frozen compilation context while
// FocusArtifacts/EntryPoints retain the explicit proof scope.
type WorkspaceRef struct {
	ID                  string                `json:"id"`
	State               WorkspaceState        `json:"state"`
	Root                string                `json:"root"`
	TreeDigest          string                `json:"tree_digest"`
	WorkingDirectory    string                `json:"working_directory"`
	BuildCommand        string                `json:"build_command"`
	Environment         []EnvironmentVariable `json:"environment"`
	EnvironmentDigest   string                `json:"environment_digest"`
	ClearEnvironment    bool                  `json:"clear_environment"`
	KillProcessGroup    bool                  `json:"kill_process_group"`
	CompilationDatabase *ArtifactRef          `json:"compilation_database,omitempty"`
	Entries             []WorkspaceEntry      `json:"entries"`
	Provenance          Provenance            `json:"provenance"`
}

// SourceLocation is a one-based, inclusive source span. A zero end position
// means that only the start position is known.
type SourceLocation struct {
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	StartColumn int    `json:"start_column"`
	EndLine     int    `json:"end_line"`
	EndColumn   int    `json:"end_column"`
}

// TranslationStatus records whether a fact was faithfully lowered. The
// aggregate values (complete, partial, blocked) are used by coverage records;
// translated/unsupported/unknown describe individual source facts.
type TranslationStatus string

const (
	TranslationUnknown     TranslationStatus = "unknown"
	TranslationTranslated  TranslationStatus = "translated"
	TranslationUnsupported TranslationStatus = "unsupported"
	TranslationComplete    TranslationStatus = "complete"
	TranslationPartial     TranslationStatus = "partial"
	TranslationBlocked     TranslationStatus = "blocked"
)

// Provenance binds one derived fact to frozen source bytes and an exact span.
// Copying the digest into each fact is deliberate: evidence remains auditable
// without relying on mutable ambient artifact state.
type Provenance struct {
	ArtifactID     string            `json:"artifact_id"`
	ArtifactDigest string            `json:"artifact_digest"`
	Location       SourceLocation    `json:"location"`
	Translation    TranslationStatus `json:"translation"`
}

// NewProvenance constructs evidence for one artifact and location.
func NewProvenance(artifact ArtifactRef, location SourceLocation, status TranslationStatus) Provenance {
	if location.Path == "" {
		location.Path = artifact.Path
	}
	return Provenance{
		ArtifactID:     artifact.ID,
		ArtifactDigest: artifact.Digest,
		Location:       location,
		Translation:    status,
	}
}

// DiagnosticSeverity controls whether compilation/translation may continue.
type DiagnosticSeverity string

const (
	SeverityInfo    DiagnosticSeverity = "info"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityError   DiagnosticSeverity = "error"
)

// DiagnosticCode is stable and machine-readable.
type DiagnosticCode string

const (
	DiagnosticInvalidInput      DiagnosticCode = "invalid-input"
	DiagnosticMissingDomain     DiagnosticCode = "missing-domain"
	DiagnosticDuplicateID       DiagnosticCode = "duplicate-id"
	DiagnosticUnreachable       DiagnosticCode = "unreachable"
	DiagnosticIncomplete        DiagnosticCode = "incomplete"
	DiagnosticOverlapping       DiagnosticCode = "overlapping"
	DiagnosticProseRequirement  DiagnosticCode = "prose-only-requirement"
	DiagnosticUnsupported       DiagnosticCode = "unsupported-construct"
	DiagnosticStaleArtifact     DiagnosticCode = "stale-artifact"
	DiagnosticInvalidProvenance DiagnosticCode = "invalid-provenance"
	DiagnosticInvalidReference  DiagnosticCode = "invalid-reference"
	DiagnosticNonFinite         DiagnosticCode = "non-finite-domain"
	// DiagnosticQuantifierAsLabel warns when a lone true/false row anchors into
	// contract text that quantifies ("every", "any", "all", "regardless"). A
	// quantified variable is a domain: writing it as one bool collapses the
	// dimension, and phase 2 can then never ask whether each value is
	// enforced -- which is exactly how a span tested only at 3 stays hidden.
	DiagnosticQuantifierAsLabel DiagnosticCode = "quantifier-as-label"
	// DiagnosticMissingBridge reports a spec whose finite model has no declared
	// bridge to the real system: an operation without a Scope or Classify
	// declaration, or a string outcome label without an Observe declaration.
	// An unbridged label is a word, and a proof over words does not transfer.
	DiagnosticMissingBridge DiagnosticCode = "missing-bridge"
)

// Diagnostic is returned instead of silently approximating invalid or
// unsupported input. Error diagnostics block proof.
type Diagnostic struct {
	Severity   DiagnosticSeverity `json:"severity"`
	Code       DiagnosticCode     `json:"code"`
	Message    string             `json:"message"`
	Provenance Provenance         `json:"provenance"`
}

// HasErrors reports whether diagnostics contain a proof-blocking error.
func HasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

// DomainValue is one spec-authored semantic label. Value is optional
// compiler-derived grounding; the strict spec deliberately does not pretend
// that a label is a concrete source-language value.
type DomainValue struct {
	ID         string           `json:"id"`
	Value      *Literal         `json:"value,omitempty"` // legacy exact grounding alias
	Groundings []GroundingAxiom `json:"groundings"`
	Provenance Provenance       `json:"provenance"`
}

type GroundingKind string

const (
	// GroundingExact is retained only for decoding older non-authoritative IR.
	// Strict spec validation accepts GroundingMembership exclusively.
	GroundingExact      GroundingKind = "exact"
	GroundingMembership GroundingKind = "membership"
)

// GroundingAxiom is frozen Phase-A semantics for one label in one operation.
// Strict v1 uses a closed boolean Membership expression for every label;
// exact cases are ordinary equality conjunctions. ConcreteWitness assigns
// every typed operation input and must satisfy Membership.
type GroundingAxiom struct {
	OperationID     string             `json:"operation_id"`
	Kind            GroundingKind      `json:"kind"`
	Exact           map[string]Literal `json:"exact,omitempty"`
	Membership      *Expression        `json:"membership,omitempty"`
	ConcreteWitness map[string]Literal `json:"concrete_witness"`
	Provenance      Provenance         `json:"provenance"`
}

// Domain is a finite, non-empty, duplicate-free set of values.
type Domain struct {
	ID         string        `json:"id"`
	Type       ValueType     `json:"type"`
	Values     []DomainValue `json:"values"`
	Provenance Provenance    `json:"provenance"`
}

// TypedValue returns compiler-derived concrete grounding when one is present.
// It never reinterprets an author label as a source-language string.
func (value DomainValue) TypedValue(domain Domain) (Literal, bool) {
	if value.Value != nil {
		return *value.Value, value.Value.Type == domain.Type || domain.Type == TypeUnknown
	}
	return Literal{}, false
}

// Assignment fixes exactly one value for every domain in a task.
type Assignment map[string]string

// Constraint excludes exactly one otherwise possible assignment. The strict
// spec compiler expands compound rows so every excluded combination has its
// own non-empty reason.
type Constraint struct {
	ID          string     `json:"id"`
	Conditions  Assignment `json:"conditions"`
	OperationID string     `json:"operation_id"`
	Reason      string     `json:"reason"`
	Provenance  Provenance `json:"provenance"`
}

// AssignmentGrounding gives one concrete reachability witness for one exact
// operation-scoped semantic assignment. It is outcome-free by design, so
// frontends may consume it without learning the row's required behavior.
type AssignmentGrounding struct {
	ID          string             `json:"id"`
	OperationID string             `json:"operation_id"`
	Conditions  Assignment         `json:"conditions"`
	Inputs      map[string]Literal `json:"inputs"`
	Provenance  Provenance         `json:"provenance"`
}

// ValueType is the finite type system used by expressions and literals.
type ValueType string

const (
	TypeUnknown  ValueType = "unknown"
	TypeBool     ValueType = "bool"
	TypeInteger  ValueType = "integer"
	TypeString   ValueType = "string"
	TypeUnit     ValueType = "unit"
	TypeSequence ValueType = "sequence"
	TypeTuple    ValueType = "tuple"
	TypeRecord   ValueType = "record"
	TypeOptional ValueType = "optional"
)

// Literal stores exactly one value selected by Type.
type Literal struct {
	Type     ValueType        `json:"type"`
	Bool     bool             `json:"bool"`
	Integer  int64            `json:"integer"`
	String   string           `json:"string"`
	Null     bool             `json:"null"`
	Elements *LiteralElements `json:"elements,omitempty"`
	Fields   *LiteralFields   `json:"fields,omitempty"`
}

// LiteralElements and LiteralFields keep Literal itself comparable (the
// scalar frontends rely on value equality) while permitting finite recursive
// composite values.
type LiteralElements struct {
	Values []Literal `json:"values"`
}

type LiteralFields struct {
	Values map[string]Literal `json:"values"`
}

// ExpressionKind is the closed expression vocabulary.
type ExpressionKind string

const (
	ExprLiteral  ExpressionKind = "literal"
	ExprVariable ExpressionKind = "variable"
	ExprUnary    ExpressionKind = "unary"
	ExprBinary   ExpressionKind = "binary"
	ExprCompare  ExpressionKind = "compare"
	ExprBool     ExpressionKind = "boolean"
	ExprCall     ExpressionKind = "call"
	ExprField    ExpressionKind = "field"
	ExprIndex    ExpressionKind = "index"
	ExprSequence ExpressionKind = "sequence"
	ExprRecord   ExpressionKind = "record"
)

// Operator is language-neutral. Frontends must reject operators without a
// corresponding constant instead of embedding source text.
type Operator string

const (
	OpNot    Operator = "not"
	OpNeg    Operator = "neg"
	OpAdd    Operator = "add"
	OpSub    Operator = "sub"
	OpMul    Operator = "mul"
	OpDiv    Operator = "div"
	OpMod    Operator = "mod"
	OpEQ     Operator = "eq"
	OpNE     Operator = "ne"
	OpLT     Operator = "lt"
	OpLE     Operator = "le"
	OpGT     Operator = "gt"
	OpGE     Operator = "ge"
	OpAnd    Operator = "and"
	OpOr     Operator = "or"
	OpIn     Operator = "in"
	OpIsNull Operator = "is-null"
)

// Expression represents literals, names, calls, and finite operators.
// Operands carries the unary operand, binary/boolean/comparison operands, or
// call arguments. Name carries a variable name or call target.
type Expression struct {
	Kind       ExpressionKind `json:"kind"`
	Type       ValueType      `json:"type"`
	Name       string         `json:"name"`
	Operator   Operator       `json:"operator"`
	Literal    *Literal       `json:"literal,omitempty"`
	Operands   []Expression   `json:"operands"`
	Provenance Provenance     `json:"provenance"`
}

// Variable declares a typed operation input. Universe is the exact finite set
// of runtime values admitted by the frozen spec when semantic labels denote
// categories rather than singleton values.
type Variable struct {
	Name       string     `json:"name"`
	Type       ValueType  `json:"type"`
	DomainID   string     `json:"domain_id"`
	Universe   []Literal  `json:"universe,omitempty"`
	Provenance Provenance `json:"provenance"`
}

// EffectKind is the finite observable side-effect vocabulary.
type EffectKind string

const (
	EffectNone   EffectKind = "none"
	EffectRead   EffectKind = "read"
	EffectWrite  EffectKind = "write"
	EffectCall   EffectKind = "call"
	EffectOutput EffectKind = "output"
)

// Effect describes an observable side effect.
type Effect struct {
	ID         string      `json:"id"`
	Kind       EffectKind  `json:"kind"`
	Target     string      `json:"target"`
	Value      *Expression `json:"value,omitempty"`
	Provenance Provenance  `json:"provenance"`
}

// StatementKind is the closed operation-body vocabulary.
type StatementKind string

const (
	StmtBranch   StatementKind = "branch"
	StmtReturn   StatementKind = "return"
	StmtRaise    StatementKind = "raise"
	StmtCall     StatementKind = "call"
	StmtEffect   StatementKind = "effect"
	StmtAssign   StatementKind = "assign"
	StmtLoop     StatementKind = "loop"
	StmtTry      StatementKind = "try"
	StmtContinue StatementKind = "continue"
)

// CatchClause is one typed exception handler in a try statement.
type CatchClause struct {
	ExceptionType string      `json:"exception_type"`
	Body          []Statement `json:"body"`
	Provenance    Provenance  `json:"provenance"`
}

// Statement models control flow and observable terminal behavior.
type Statement struct {
	Kind          StatementKind `json:"kind"`
	Target        string        `json:"target"`
	Condition     *Expression   `json:"condition,omitempty"`
	Iterator      *Expression   `json:"iterator,omitempty"`
	Value         *Expression   `json:"value,omitempty"`
	ExceptionType string        `json:"exception_type"`
	Message       string        `json:"message"`
	Then          []Statement   `json:"then"`
	Else          []Statement   `json:"else"`
	Catches       []CatchClause `json:"catches"`
	Effects       []Effect      `json:"effects"`
	Provenance    Provenance    `json:"provenance"`
}

// OperationKind distinguishes functions, methods, and whole test cases.
type OperationKind string

const (
	OperationCallable OperationKind = "callable"
	OperationFunction OperationKind = "function"
	OperationMethod   OperationKind = "method"
	OperationTest     OperationKind = "test"
)

// Operation is a bounded callable. Body is optional audit detail and is never
// a substitute for compiler-IR behavior/realization evidence.
type Operation struct {
	ID         string        `json:"id"`
	Kind       OperationKind `json:"kind"`
	DomainIDs  []string      `json:"domain_ids"`
	OutcomeIDs []string      `json:"outcome_ids"`
	Inputs     []Variable    `json:"inputs"`
	Body       []Statement   `json:"body"`
	Provenance Provenance    `json:"provenance"`
}

// OutcomeKind is an externally observable terminal result.
type OutcomeKind string

const (
	OutcomeReturn  OutcomeKind = "return"
	OutcomeRaise   OutcomeKind = "raise"
	OutcomeSuccess OutcomeKind = "success"
	// OutcomeTimeout is emitted only by the frozen execution environment after
	// the declared operation deadline expires; solution output cannot
	// self-report this terminal.
	OutcomeTimeout OutcomeKind = "timeout"
	OutcomeOther   OutcomeKind = "other"
)

// ObservableOutcome is a typed member of the finite behavior universe.
// ID is the stable comparison key used by requirements, code cases, and tests.
type ObservableOutcome struct {
	ID            string      `json:"id"`
	Kind          OutcomeKind `json:"kind"`
	Value         *Literal    `json:"value,omitempty"`
	ExceptionType string      `json:"exception_type"`
	Message       string      `json:"message"`
	OperationID   string      `json:"operation_id"`
	Effects       []Effect    `json:"effects"`
	Provenance    Provenance  `json:"provenance"`
}

// BehaviorCase enumerates outcomes emitted by translated code for one exact
// concrete behavior point. Conditions identify the semantic category;
// Inputs distinguishes implementations that behave differently within it.
type BehaviorCase struct {
	ID          string             `json:"id"`
	Conditions  Assignment         `json:"conditions"`
	OperationID string             `json:"operation_id"`
	Inputs      map[string]Literal `json:"inputs"`
	OutcomeIDs  []string           `json:"outcome_ids"`
	Provenance  Provenance         `json:"provenance"`
}

// RawReferenceCase is independently extracted reference behavior before any
// comparison with Spec requirements. Outcomes contain runtime facts only;
// NormalizeReferenceCases performs the sole mechanical mapping into the
// frozen observable alphabet O.
type RawReferenceCase struct {
	ID          string             `json:"id"`
	Conditions  Assignment         `json:"conditions"`
	OperationID string             `json:"operation_id"`
	Inputs      map[string]Literal `json:"inputs"`
	Outcomes    []RawOutcomeTrace  `json:"outcomes"`
	Provenance  Provenance         `json:"provenance"`
}

// Invariant is an explicit typed predicate, not prose.
type Invariant struct {
	ID         string             `json:"id"`
	Predicate  Expression         `json:"predicate"`
	Bindings   []InvariantBinding `json:"bindings"`
	Provenance Provenance         `json:"provenance"`
}

// InvariantBindingKind selects a value in the current assignment/outcome.
type InvariantBindingKind string

const (
	BindDomainValue  InvariantBindingKind = "domain-value"
	BindOutcomeValue InvariantBindingKind = "outcome-value"
	BindEffectValue  InvariantBindingKind = "effect-value"
)

// InvariantBinding supplies a predicate variable from finite behavior state.
type InvariantBinding struct {
	Variable     string               `json:"variable"`
	Kind         InvariantBindingKind `json:"kind"`
	DomainID     string               `json:"domain_id"`
	EffectKind   EffectKind           `json:"effect_kind"`
	EffectTarget string               `json:"effect_target"`
	Provenance   Provenance           `json:"provenance"`
}

// AssertionKind is the closed test-oracle vocabulary.
type AssertionKind string

const (
	AssertEqual     AssertionKind = "equal"
	AssertNotEqual  AssertionKind = "not-equal"
	AssertTrue      AssertionKind = "true"
	AssertFalse     AssertionKind = "false"
	AssertRaises    AssertionKind = "raises"
	AssertOutcomeIn AssertionKind = "outcome-in"
)

// Assertion describes what a test actually observes.
type Assertion struct {
	Kind          AssertionKind `json:"kind"`
	Actual        *Expression   `json:"actual,omitempty"`
	Expected      *Expression   `json:"expected,omitempty"`
	ExceptionType string        `json:"exception_type"`
	Message       string        `json:"message"`
	OutcomeIDs    []string      `json:"outcome_ids"`
	Provenance    Provenance    `json:"provenance"`
}

// TestModel gives the accepted outcome set for one exact assignment.
type TestModel struct {
	ID               string        `json:"id"`
	Conditions       Assignment    `json:"conditions"`
	OperationID      string        `json:"operation_id"`
	Assertions       []Assertion   `json:"assertions"`
	AcceptedOutcomes []string      `json:"accepted_outcomes"`
	Predicate        TestPredicate `json:"predicate"`
	Provenance       Provenance    `json:"provenance"`
}

// ArtifactModelDigest binds one complete independently translated artifact
// model or projection component into aggregate test-suite evidence.
type ArtifactModelDigest struct {
	ArtifactID string `json:"artifact_id"`
	Digest     string `json:"digest"`
}

type TestBehaviorEquality struct {
	Behavior  BehaviorRef       `json:"behavior"`
	Predicate CompilerPredicate `json:"predicate"`
}

// TestObservationCompleteness proves that verifier pass/fail is extensional
// in the modeled behavior vector: two concrete implementations with equal
// BehaviorChoice vectors cannot receive different pass signals.
type TestObservationCompleteness struct {
	Components            []TestExtensionalityComponent `json:"components"`
	ProjectionComponents  []ArtifactModelDigest         `json:"projection_components"`
	SourceModels          []ArtifactModelDigest         `json:"source_models"`
	StaticPredicateDigest string                        `json:"static_predicate_digest"`
	IRKind                CompilerIRKind                `json:"ir_kind"`
	Constructs            []TestConstructEvidence       `json:"constructs"`
	ObservationIRDigest   string                        `json:"observation_ir_digest"`
	HarnessDigest         string                        `json:"harness_digest"`
	Prover                ToolRef                       `json:"prover"`
	BehaviorEqualities    []TestBehaviorEquality        `json:"behavior_equalities"`
	LeftPass              CompilerPredicate             `json:"left_pass"`
	RightPass             CompilerPredicate             `json:"right_pass"`
	Result                ProofResult                   `json:"result"`
	ProofDigest           string                        `json:"proof_digest"`
	Proof                 ReplayableProof               `json:"proof"`
	Provenance            Provenance                    `json:"provenance"`
}

type TestExtensionalityEvidence struct {
	ObservationIRDigest string                 `json:"observation_ir_digest"`
	HarnessDigest       string                 `json:"harness_digest"`
	Prover              ToolRef                `json:"prover"`
	BehaviorEqualities  []TestBehaviorEquality `json:"behavior_equalities"`
	LeftPass            CompilerPredicate      `json:"left_pass"`
	RightPass           CompilerPredicate      `json:"right_pass"`
	Result              ProofResult            `json:"result"`
	ProofDigest         string                 `json:"proof_digest"`
	Proof               ReplayableProof        `json:"proof"`
	Provenance          Provenance             `json:"provenance"`
}

type TestExtensionalityComponent struct {
	ArtifactID string                     `json:"artifact_id"`
	Digest     string                     `json:"digest"`
	Evidence   TestExtensionalityEvidence `json:"evidence"`
}

type TestConstructKind string

const (
	TestConstructFixture   TestConstructKind = "fixture"
	TestConstructParameter TestConstructKind = "parameter"
	TestConstructControl   TestConstructKind = "control"
	TestConstructAssertion TestConstructKind = "assertion"
	TestConstructMock      TestConstructKind = "mock"
	TestConstructEffect    TestConstructKind = "effect"
	TestConstructCall      TestConstructKind = "call"
)

// TestConstructEvidence binds every construct contributing to the static
// TestsPass predicate to immutable compiler/interpreter IR nodes.
type TestConstructEvidence struct {
	ID              string            `json:"id"`
	ArtifactID      string            `json:"artifact_id"`
	Kind            TestConstructKind `json:"kind"`
	Digest          string            `json:"digest"`
	IRKind          CompilerIRKind    `json:"ir_kind"`
	IRDigest        string            `json:"ir_digest"`
	Tool            ToolRef           `json:"tool"`
	CompilerNodeIDs []string          `json:"compiler_node_ids"`
	Provenance      Provenance        `json:"provenance"`
}

// TestSuiteModel is the sole proof truth for TestsPass. Its predicate is the
// deterministic conjunction of complete compiler-derived per-test artifact
// projection graphs. Vector execution is optional cross-check evidence only.
type TestSuiteModel struct {
	SourceArtifacts         []ArtifactRef               `json:"source_artifacts"`
	SourceModels            []ArtifactModelDigest       `json:"source_models"`
	Predicate               TestPredicate               `json:"predicate"`
	Verifier                ToolRef                     `json:"verifier"`
	Execution               WorkspaceCommand            `json:"execution"`
	Vectors                 []TestVectorResult          `json:"vectors"`
	VectorCount             uint64                      `json:"vector_count"`
	AcceptedVectorCount     uint64                      `json:"accepted_vector_count"`
	AcceptedVectorsDigest   string                      `json:"accepted_vectors_digest"`
	VectorEvidenceDigest    string                      `json:"vector_evidence_digest"`
	Repetitions             int                         `json:"repetitions"`
	RunDigests              []string                    `json:"run_digests"`
	CrossCheck              *TestCrossCheckEvidence     `json:"cross_check,omitempty"`
	RunnerComposition       RunnerCompositionEvidence   `json:"runner_composition"`
	ObservationCompleteness TestObservationCompleteness `json:"observation_completeness"`
	Coverage                TranslationCoverage         `json:"coverage"`
	Evidence                []Provenance                `json:"evidence"`
}

// TestCrossCheckEvidence is optional execution evidence over a declared
// subset of behavior vectors. Full=true additionally proves all mathematical
// candidate vectors were checked; it is never the authority for TestsPass.
type TestCrossCheckEvidence struct {
	Full                  bool               `json:"full"`
	Vectors               []TestVectorResult `json:"vectors"`
	AcceptedVectorCount   uint64             `json:"accepted_vector_count"`
	AcceptedVectorsDigest string             `json:"accepted_vectors_digest"`
	VectorEvidenceDigest  string             `json:"vector_evidence_digest"`
	Repetitions           int                `json:"repetitions"`
	RunDigests            []string           `json:"run_digests"`
	Provenance            Provenance         `json:"provenance"`
}

// InstructionClause identifies one reviewed slice of the frozen instruction.
type InstructionClause struct {
	ID          string         `json:"id"`
	Span        SourceLocation `json:"span"`
	SliceDigest string         `json:"slice_digest"`
	Provenance  Provenance     `json:"provenance"`
}

// InstructionModel is the independently frozen instruction-side model.
type InstructionModel struct {
	Artifact ArtifactRef         `json:"artifact"`
	Clauses  []InstructionClause `json:"clauses"`
	Coverage TranslationCoverage `json:"coverage"`
}

type SpecAcceptanceDecision string

const (
	SpecAcceptanceAccepted SpecAcceptanceDecision = "accepted"
	SpecAcceptanceRejected SpecAcceptanceDecision = "rejected"
)

type AcceptanceSourceBinding struct {
	Role     string `json:"role"`
	Path     string `json:"path"`
	Digest   string `json:"digest"`
	Relevant string `json:"relevant"`
}

type AcceptanceDomainBinding struct {
	OperationID string                   `json:"operation_id"`
	DomainID    string                   `json:"domain_id"`
	ValueIDs    []string                 `json:"value_ids"`
	Labels      []AcceptanceLabelBinding `json:"labels"`
}

type AcceptanceLabelBinding struct {
	ValueID                  string       `json:"value_id"`
	DefinitionEvidence       []Provenance `json:"definition_evidence"`
	ExpectedCompilerPath     string       `json:"expected_compiler_path"`
	ExpectedReachableWitness string       `json:"expected_reachable_witness"`
}

type AcceptanceOperationBinding struct {
	OperationID          string                 `json:"operation_id"`
	EntryPoint           string                 `json:"entry_point"`
	PhaseAEvidence       string                 `json:"phase_a_evidence"`
	ObservableBoundary   string                 `json:"observable_boundary"`
	InstructionClauseIDs []string               `json:"instruction_clause_ids"`
	Evidence             []Provenance           `json:"evidence"`
	Decision             SpecAcceptanceDecision `json:"decision"`
}

type AcceptanceConstraintBinding struct {
	ID             string       `json:"id"`
	OperationID    string       `json:"operation_id"`
	Conditions     Assignment   `json:"conditions"`
	Reason         string       `json:"reason"`
	NoPathEvidence string       `json:"no_path_evidence"`
	Evidence       []Provenance `json:"evidence"`
}

type AcceptanceReviewBinding struct {
	ID                   string                 `json:"id"`
	RequirementIDs       []string               `json:"requirement_ids"`
	InstructionClauseIDs []string               `json:"instruction_clause_ids"`
	Decision             SpecAcceptanceDecision `json:"decision"`
	Evidence             []Provenance           `json:"evidence"`
}
type AcceptanceResolution struct {
	ID           string                 `json:"id"`
	SourceRoles  []string               `json:"source_roles"`
	Disagreement string                 `json:"disagreement"`
	Resolution   string                 `json:"resolution"`
	Decision     SpecAcceptanceDecision `json:"decision"`
	Evidence     []Provenance           `json:"evidence"`
}

// SpecAcceptanceEvidence is the typed, frozen Phase-A human review boundary.
// It is an attestation that author/reviewer accepted the independently
// authored spec, not a machine proof that instruction prose entails it.
type SpecAcceptanceEvidence struct {
	Schema                  string                        `json:"schema"`
	AuthoringRecord         ArtifactRef                   `json:"authoring_record"`
	DetachedLedger          ArtifactRef                   `json:"detached_ledger"`
	PhaseASpec              ArtifactRef                   `json:"phase_a_spec"`
	PhaseAEnvironment       ArtifactRef                   `json:"phase_a_environment"`
	PhaseAEnvironmentModel  PhaseAEnvironmentModel        `json:"phase_a_environment_model"`
	FinalSpec               ArtifactRef                   `json:"final_spec"`
	Instruction             ArtifactRef                   `json:"instruction"`
	Environment             ArtifactRef                   `json:"environment"`
	TaskID                  string                        `json:"task_id"`
	PhaseASpecIRDigest      string                        `json:"phase_a_spec_ir_digest"`
	FrozenSemanticsDigest   string                        `json:"frozen_semantics_digest"`
	EnvironmentConfigDigest string                        `json:"environment_config_digest"`
	Manifest                []AcceptanceSourceBinding     `json:"manifest"`
	Operations              []AcceptanceOperationBinding  `json:"operations"`
	OperationIDs            []string                      `json:"operation_ids,omitempty"` // legacy decode only; strict evidence uses Operations
	Domains                 []AcceptanceDomainBinding     `json:"domains"`
	Constraints             []AcceptanceConstraintBinding `json:"constraints"`
	Reviews                 []AcceptanceReviewBinding     `json:"reviews"`
	Resolutions             []AcceptanceResolution        `json:"resolutions"`
	NoDisagreements         bool                          `json:"no_disagreements"`
	ConstraintIDs           []string                      `json:"constraint_ids,omitempty"` // legacy decode only; strict evidence uses Constraints
	LintCommand             string                        `json:"lint_command"`
	TestAccess              string                        `json:"test_access"`
	Decision                SpecAcceptanceDecision        `json:"decision"`
	ExpandedTableReview     SpecAcceptanceDecision        `json:"expanded_table_review"`
	ExpectedGroundingReview SpecAcceptanceDecision        `json:"expected_grounding_review"`
	AuthorIdentity          string                        `json:"author_identity"`
	IndependentReviewer     string                        `json:"independent_reviewer"`
	CompletedAtUTC          string                        `json:"completed_at_utc"`
	SnapshotPath            string                        `json:"snapshot_path"`
	FinalPath               string                        `json:"final_path"`
	LedgerPath              string                        `json:"ledger_path"`
	Complete                bool                          `json:"complete"`
	Evidence                []Provenance                  `json:"evidence"`
}

const PhaseAEnvironmentSchemaV1 = "hyperray.phase-a-environment/v1"

// PhaseAEnvironmentModel is the canonical test-blind environment subset used
// during spec authoring. It deliberately contains no test command or wiring.
type PhaseAEnvironmentModel struct {
	Schema              string        `json:"schema"`
	Identity            string        `json:"identity"`
	ConfigurationDigest string        `json:"configuration_digest"`
	Tools               []ToolRef     `json:"tools"`
	SourceArtifacts     []ArtifactRef `json:"source_artifacts"`
	Complete            bool          `json:"complete"`
}

// BehaviorRef names one concrete component of the global candidate behavior
// vector. Inputs is the complete typed operation-input map. A nil Inputs map
// is a category reference and is valid only in evidence records that prove a
// singleton grounding or a universally quantified symbolic quotient; test
// observations and concrete choices always carry Inputs.
type BehaviorRef struct {
	OperationID string             `json:"operation_id"`
	Conditions  Assignment         `json:"conditions"`
	Inputs      map[string]Literal `json:"inputs"`
	Provenance  Provenance         `json:"provenance"`
}

// BehaviorChoice is one selected outcome in a complete candidate behavior
// vector.
type BehaviorChoice struct {
	Behavior  BehaviorRef `json:"behavior"`
	OutcomeID string      `json:"outcome_id"`
}

// ObservationKind selects an observable projection of a behavior choice.
type ObservationKind string

const (
	ObserveOutcome ObservationKind = "outcome"
	ObserveRaise   ObservationKind = "raise"
	ObserveEffect  ObservationKind = "effect"
)

// Observation is a typed test-oracle leaf.
type Observation struct {
	Kind          ObservationKind `json:"kind"`
	Behavior      BehaviorRef     `json:"behavior"`
	OutcomeIDs    []string        `json:"outcome_ids"`
	ExceptionType string          `json:"exception_type"`
	Message       string          `json:"message"`
	EffectKind    EffectKind      `json:"effect_kind"`
	EffectTarget  string          `json:"effect_target"`
	EffectValue   *Expression     `json:"effect_value,omitempty"`
	Provenance    Provenance      `json:"provenance"`
}

// TestPredicateKind is the closed global test-predicate vocabulary.
type TestPredicateKind string

const (
	PredicateTrue         TestPredicateKind = "true"
	PredicateFalse        TestPredicateKind = "false"
	PredicateAnd          TestPredicateKind = "and"
	PredicateOr           TestPredicateKind = "or"
	PredicateNot          TestPredicateKind = "not"
	PredicateOutcomeIn    TestPredicateKind = "outcome-in"
	PredicateOutcomeEqual TestPredicateKind = "outcome-equal"
	PredicateRaises       TestPredicateKind = "raises"
	PredicateHasEffect    TestPredicateKind = "has-effect"
)

// TestPredicate can relate any number of calls/cases. OutcomeEqual compares
// the selected outcome IDs for Left and Right; other leaf kinds use Observe.
type TestPredicate struct {
	Kind       TestPredicateKind `json:"kind"`
	Children   []TestPredicate   `json:"children"`
	Observe    *Observation      `json:"observe,omitempty"`
	Left       *BehaviorRef      `json:"left,omitempty"`
	Right      *BehaviorRef      `json:"right,omitempty"`
	Provenance Provenance        `json:"provenance"`
}

// RequirementCase is the specification for one exact reachable assignment.
// Required is non-empty; Required and Forbidden are disjoint and together
// cover the operation's outcome universe. Forbidden is empty when every
// declared outcome is allowed.
type RequirementCase struct {
	ID                   string       `json:"id"`
	Conditions           Assignment   `json:"conditions"`
	OperationID          string       `json:"operation_id"`
	RequiredOutcomes     []string     `json:"required_outcomes"`
	ForbiddenOutcomes    []string     `json:"forbidden_outcomes"`
	Effects              []Effect     `json:"effects"`
	InvariantIDs         []string     `json:"invariant_ids"`
	TestIDs              []string     `json:"test_ids"`
	GroundingID          string       `json:"grounding_id"`
	InstructionClauseIDs []string     `json:"instruction_clause_ids"`
	InstructionSources   []Provenance `json:"instruction_sources"`
	Evidence             []Provenance `json:"evidence"`
	Provenance           Provenance   `json:"provenance"`
}

// UnsupportedConstruct records every source construct a frontend could not
// faithfully lower. Its presence makes aggregate coverage blocked.
type UnsupportedConstruct struct {
	Kind       string     `json:"kind"`
	Reason     string     `json:"reason"`
	Provenance Provenance `json:"provenance"`
}

// TranslationCoverage makes silent approximation structurally visible.
type TranslationCoverage struct {
	Status               TranslationStatus      `json:"status"`
	TotalConstructs      int                    `json:"total_constructs"`
	TranslatedConstructs int                    `json:"translated_constructs"`
	Unsupported          []UnsupportedConstruct `json:"unsupported"`
	Provenance           Provenance             `json:"provenance"`
}

// FrontendRequest is the common contract implemented by every language
// package's Translate function. FiniteDomains, Constraints, Operations,
// Outcomes are proof scope copied from the compiled frozen spec, never from
// mutable frontend configuration or inferred from source behavior.
type FrontendRequest struct {
	TaskID         string                `json:"task_id"`
	Artifact       ArtifactRef           `json:"artifact"`
	Language       Language              `json:"language"`
	Kind           ArtifactKind          `json:"kind"`
	Source         []byte                `json:"source"`
	SourceRanges   []SourceRoleRange     `json:"source_ranges"`
	EntryPoints    []string              `json:"entry_points"`
	FiniteDomains  []Domain              `json:"finite_domains"`
	Groundings     []AssignmentGrounding `json:"groundings"`
	Constraints    []Constraint          `json:"constraints"`
	Operations     []Operation           `json:"operations"`
	Outcomes       []ObservableOutcome   `json:"outcomes"`
	Options        map[string]string     `json:"options"`
	Translator     ToolRef               `json:"translator"`
	Prover         ToolRef               `json:"prover"`
	Runner         ToolRef               `json:"runner"`
	RunnerCommand  *WorkspaceCommand     `json:"runner_command,omitempty"`
	Configuration  *ArtifactRef          `json:"configuration,omitempty"`
	Workspace      WorkspaceRef          `json:"workspace"`
	FocusArtifacts []ArtifactRef         `json:"focus_artifacts"`
	ChangedRanges  []ChangedSourceRange  `json:"changed_ranges"`
}

// ArtifactModel is a complete translation of one independent frozen input.
type ArtifactModel struct {
	Artifact           ArtifactRef                   `json:"artifact"`
	Language           Language                      `json:"language"`
	Kind               ArtifactKind                  `json:"kind"`
	SourceRanges       []SourceRoleRange             `json:"source_ranges"`
	Domains            []Domain                      `json:"domains"`
	Groundings         []AssignmentGrounding         `json:"groundings"`
	Constraints        []Constraint                  `json:"constraints"`
	Operations         []Operation                   `json:"operations"`
	Outcomes           []ObservableOutcome           `json:"outcomes"`
	RawReferenceCases  []RawReferenceCase            `json:"raw_reference_cases"`
	Cases              []BehaviorCase                `json:"cases"`
	Invariants         []Invariant                   `json:"invariants"`
	Tests              []TestModel                   `json:"tests"`
	CompilerEvidence   []CompilerEvidence            `json:"compiler_evidence"`
	ExhaustiveEvidence []ExhaustiveExecutionEvidence `json:"exhaustive_evidence"`
	ScopeClosure       *ScopeClosureEvidence         `json:"scope_closure"`
	TestProjection     *TestObservationProjection    `json:"test_projection"`
	RunnerSelection    *RunnerSelectionEvidence      `json:"runner_selection"`
	Coverage           TranslationCoverage           `json:"coverage"`
	Translator         ToolRef                       `json:"translator"`
}

type TestDependencyKind string

const (
	TestDependencyCall   TestDependencyKind = "call"
	TestDependencyRead   TestDependencyKind = "read"
	TestDependencyEffect TestDependencyKind = "effect"
)

type TestBehaviorDependency struct {
	ConstructID     string             `json:"construct_id"`
	Kind            TestDependencyKind `json:"kind"`
	Behavior        BehaviorRef        `json:"behavior"`
	Inputs          map[string]Literal `json:"inputs"`
	CompilerNodeIDs []string           `json:"compiler_node_ids"`
	Provenance      Provenance         `json:"provenance"`
}

type CompilerDerivationEvidence struct {
	SourceDigest        string         `json:"source_digest"`
	WorkspaceTreeDigest string         `json:"workspace_tree_digest"`
	Tool                ToolRef        `json:"tool"`
	IRKind              CompilerIRKind `json:"ir_kind"`
	IRDigest            string         `json:"ir_digest"`
	Steps               []ProbeStep    `json:"steps"`
	Output              []byte         `json:"output"`
	OutputDigest        string         `json:"output_digest"`
	DecodedModelDigest  string         `json:"decoded_model_digest"`
	Complete            bool           `json:"complete"`
}

// TestObservationProjection is the per-artifact compiler/interpreter-derived
// proof that every pass-influencing construct depends only on modeled
// BehaviorRefs. Aggregate Test IR may compose these records but not replace
// them with an invented predicate.
type TestObservationProjection struct {
	Source          ArtifactRef                  `json:"source"`
	TestIDs         []string                     `json:"test_ids"`
	PredicateDigest string                       `json:"predicate_digest"`
	Constructs      []TestConstructEvidence      `json:"constructs"`
	Dependencies    []TestBehaviorDependency     `json:"dependencies"`
	Nodes           []TestProjectionNode         `json:"nodes"`
	PassRoots       []TestPassRoot               `json:"pass_roots"`
	Quantification  []TestQuantificationEvidence `json:"quantification"`
	Derivation      CompilerDerivationEvidence   `json:"derivation"`
	Extensionality  TestExtensionalityEvidence   `json:"extensionality"`
	Complete        bool                         `json:"complete"`
	Provenance      Provenance                   `json:"provenance"`
}

type TestQuantificationKind string

const (
	TestQuantificationSingleton        TestQuantificationKind = "singleton-grounding"
	TestQuantificationFiniteExhaustive TestQuantificationKind = "finite-concrete-exhaustive"
	TestQuantificationUniversalGraph   TestQuantificationKind = "universal-compiler-graph"
)

// TestQuantificationEvidence states how a semantic category expands into
// concrete proof points. ConcreteInputs is the complete point set for
// singleton/finite modes; it never licenses one shared category result.
// Universal graph mode is reserved for central symbolic function semantics;
// opaque frontend SMT cannot populate it.
type TestQuantificationEvidence struct {
	Behavior             BehaviorRef            `json:"behavior"`
	Kind                 TestQuantificationKind `json:"kind"`
	ConcreteInputs       []map[string]Literal   `json:"concrete_inputs"`
	ConcreteInputsDigest string                 `json:"concrete_inputs_digest"`
	CodeGraphDigest      string                 `json:"code_graph_digest"`
	TestGraphDigest      string                 `json:"test_graph_digest"`
	Result               ProofResult            `json:"result"`
	Provenance           Provenance             `json:"provenance"`
}

type TestPassRoot struct {
	TestID          string   `json:"test_id"`
	NodeID          string   `json:"node_id"`
	CompilerNodeIDs []string `json:"compiler_node_ids"`
}

// TestProjectionNode is a closed compiler-IR dependency graph. Internal
// nodes are AND/OR/NOT; leaves are constants or typed BehaviorRef
// observations. No external/unclassified leaf can be represented.
type TestProjectionNode struct {
	ID              string            `json:"id"`
	Kind            TestPredicateKind `json:"kind"`
	Children        []string          `json:"children"`
	Observe         *Observation      `json:"observe,omitempty"`
	Left            *BehaviorRef      `json:"left,omitempty"`
	Right           *BehaviorRef      `json:"right,omitempty"`
	CompilerNodeIDs []string          `json:"compiler_node_ids"`
	ConstructIDs    []string          `json:"construct_ids"`
	Provenance      Provenance        `json:"provenance"`
}

// RunnerSelectionEvidence proves the frozen direct runner selects exactly
// the translated tests and defines suite pass as their conjunction.
type RunnerSelectionEvidence struct {
	TestIDs         []string         `json:"test_ids"`
	PredicateDigest string           `json:"predicate_digest"`
	Configuration   ArtifactRef      `json:"configuration"`
	Verifier        ToolRef          `json:"verifier"`
	Command         WorkspaceCommand `json:"command"`
	ConjunctivePass bool             `json:"conjunctive_pass"`
	Complete        bool             `json:"complete"`
	Provenance      Provenance       `json:"provenance"`
}

type RunnerCompositionKind string

const (
	RunnerCompositionConjunction     RunnerCompositionKind = "conjunction"
	RunnerCompositionOrderedStateful RunnerCompositionKind = "ordered-stateful"
)

type RunnerEventKind string

const (
	RunnerEventSetup    RunnerEventKind = "setup"
	RunnerEventTest     RunnerEventKind = "test"
	RunnerEventTeardown RunnerEventKind = "teardown"
)

// RunnerCompositionComponent binds one independently translated verifier
// artifact and its local compiler-derived test selection into the real global
// runner, without requiring the local analysis command to equal the global
// grading command.
type RunnerCompositionComponent struct {
	ArtifactID      string   `json:"artifact_id"`
	ArtifactDigest  string   `json:"artifact_digest"`
	ModelDigest     string   `json:"model_digest"`
	SelectionDigest string   `json:"selection_digest"`
	TestIDs         []string `json:"test_ids"`
}

// RunnerStateObject is one finite shared setup/runtime state included in the
// verifier compilation. InitialValue is nil only for compiler-defined unit
// state; all reads/writes are named by RunnerEvent.
type RunnerStateObject struct {
	ID              string     `json:"id"`
	InitialValue    *Literal   `json:"initial_value,omitempty"`
	CompilerNodeIDs []string   `json:"compiler_node_ids"`
	Provenance      Provenance `json:"provenance"`
}

// RunnerEvent records the exact execution order selected by the frozen global
// runner, including setup/teardown and shared-state dependencies.
type RunnerEvent struct {
	Ordinal         int             `json:"ordinal"`
	ID              string          `json:"id"`
	Kind            RunnerEventKind `json:"kind"`
	ArtifactID      string          `json:"artifact_id"`
	TestID          string          `json:"test_id"`
	ReadsStateIDs   []string        `json:"reads_state_ids"`
	WritesStateIDs  []string        `json:"writes_state_ids"`
	CompilerNodeIDs []string        `json:"compiler_node_ids"`
	Provenance      Provenance      `json:"provenance"`
}

// RunnerCompositionEvidence is the global T(F) composition assembled from
// independently compiler-derived test projections and runner-selection
// records: exact grading command/pass signal, ordered selected tests, and
// setup/state semantics. No test name or source-text convention is accepted
// as authority.
type RunnerCompositionEvidence struct {
	Kind            RunnerCompositionKind        `json:"kind"`
	SourceArtifacts []ArtifactRef                `json:"source_artifacts"`
	Components      []RunnerCompositionComponent `json:"components"`
	States          []RunnerStateObject          `json:"states"`
	Events          []RunnerEvent                `json:"events"`
	PredicateDigest string                       `json:"predicate_digest"`
	Verifier        ToolRef                      `json:"verifier"`
	Execution       WorkspaceCommand             `json:"execution"`
	// Derivation is retained for certificate compatibility. Global composition
	// authority comes from the validated per-artifact compiler projections and
	// runner selections; this optional envelope is never used to prove them.
	Derivation CompilerDerivationEvidence `json:"derivation,omitempty"`
	Digest     string                     `json:"digest"`
	Complete   bool                       `json:"complete"`
	Provenance Provenance                 `json:"provenance"`
}

// CompilerIRKind identifies the immutable executable semantics a language
// frontend projected. Proof never assigns semantics to high-level source AST
// nodes; it relies on these compiler/interpreter artifacts and their finite
// path partitions.
type CompilerIRKind string

const (
	CompilerIRCPythonBytecode CompilerIRKind = "cpython-bytecode"
	CompilerIRRustMIR         CompilerIRKind = "rust-mir"
	CompilerIRLLVM            CompilerIRKind = "llvm-ir"
	CompilerIRVerifierGraph   CompilerIRKind = "verifier-pass-graph"
)

type CompilerEvidenceMethod string

const CompilerEvidenceModelChecker CompilerEvidenceMethod = "compiler-model-checker"

type CompilerDeclaration struct {
	ID              string         `json:"id"`
	QualifiedName   string         `json:"qualified_name"`
	Artifact        ArtifactRef    `json:"artifact"`
	Location        SourceLocation `json:"location"`
	CompilerNodeIDs []string       `json:"compiler_node_ids"`
	Changed         bool           `json:"changed"`
	Provenance      Provenance     `json:"provenance"`
}

type ResolvedCallEdge struct {
	CallerDeclarationID string         `json:"caller_declaration_id"`
	CalleeDeclarationID string         `json:"callee_declaration_id"`
	Location            SourceLocation `json:"location"`
	CompilerNodeID      string         `json:"compiler_node_id"`
	Provenance          Provenance     `json:"provenance"`
}

type OperationOwner struct {
	OperationID   string `json:"operation_id"`
	DeclarationID string `json:"declaration_id"`
}

// ScopeClosureEvidence is the compiler-derived changed-declaration and
// transitive caller closure used to establish exact proof scope.
type ScopeClosureEvidence struct {
	SourceArtifacts        []ArtifactRef         `json:"source_artifacts"`
	WorkspaceTreeDigest    string                `json:"workspace_tree_digest"`
	Compiler               ToolRef               `json:"compiler"`
	Prover                 ToolRef               `json:"prover"`
	CompilerIRDigest       string                `json:"compiler_ir_digest"`
	ChangedRanges          []ChangedSourceRange  `json:"changed_ranges"`
	Declarations           []CompilerDeclaration `json:"declarations"`
	CallEdges              []ResolvedCallEdge    `json:"call_edges"`
	ImpactedDeclarationIDs []string              `json:"impacted_declaration_ids"`
	OperationOwners        []OperationOwner      `json:"operation_owners"`
	Completeness           ProofResult           `json:"completeness"`
	CompletenessProof      ReplayableProof       `json:"completeness_proof"`
	Complete               bool                  `json:"complete"`
	Provenance             Provenance            `json:"provenance"`
}

// ProofResult is a closed status for compiler-path proof evidence. Anything
// other than proved blocks VERIFIED.
type ProofResult string

const (
	ProofProved  ProofResult = "proved"
	ProofUnknown ProofResult = "unknown"
	ProofRefuted ProofResult = "refuted"
)

// ProofLogic and SolverResult make compiler-path proofs independently
// replayable rather than accepting a frontend's self-asserted proved flag.
type ProofLogic string

const ProofLogicSMTLIB2 ProofLogic = "smtlib2"

type SolverResult string

const (
	SolverSAT     SolverResult = "sat"
	SolverUNSAT   SolverResult = "unsat"
	SolverUnknown SolverResult = "unknown"
)

type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ReplayableProof binds exact solver input/output bytes and invocation. Query
// is supplied on stdin to Prover.Path with Argv; a verifier reruns it and
// requires byte-identical output plus the declared result.
type ReplayableProof struct {
	Claim              ProofClaim            `json:"claim"`
	Logic              ProofLogic            `json:"logic"`
	Query              []byte                `json:"query"`
	QueryDigest        string                `json:"query_digest"`
	Prover             ToolRef               `json:"prover"`
	Argv               []string              `json:"argv"`
	WorkingDirectory   string                `json:"working_directory"`
	Environment        []EnvironmentVariable `json:"environment"`
	EnvironmentDigest  string                `json:"environment_digest"`
	ClearEnvironment   bool                  `json:"clear_environment"`
	KillProcessGroup   bool                  `json:"kill_process_group"`
	TimeoutMillis      int64                 `json:"timeout_millis"`
	SolverOutput       []byte                `json:"solver_output"`
	SolverOutputDigest string                `json:"solver_output_digest"`
	Result             SolverResult          `json:"result"`
	SubjectDigests     []string              `json:"subject_digests"`
}

// CompilerPredicate is exact SMT-LIB2 predicate text projected from immutable
// compiler IR. Its digest is referenced by replayable proof subjects.
type CompilerPredicate struct {
	Logic              ProofLogic `json:"logic"`
	Declarations       []byte     `json:"declarations"`
	DeclarationsDigest string     `json:"declarations_digest"`
	Formula            []byte     `json:"formula"`
	FormulaDigest      string     `json:"formula_digest"`
	Tool               ToolRef    `json:"tool"`
	IRDigest           string     `json:"ir_digest"`
	CompilerNodeIDs    []string   `json:"compiler_node_ids"`
}

// CompilerProofContext freezes every input that gives a compiler predicate
// its meaning. The canonical query binds these digests in addition to the
// predicate text; the containing CompilerEvidence must match them exactly.
type CompilerProofContext struct {
	SourceDigest        string  `json:"source_digest"`
	WorkspaceTreeDigest string  `json:"workspace_tree_digest"`
	EmittedIRDigest     string  `json:"emitted_ir_digest"`
	HarnessDigest       string  `json:"harness_digest"`
	Compiler            ToolRef `json:"compiler"`
}

// CompilerOutcomePredicate is a compiler-IR predicate saying that the
// observable terminal/effect trace is exactly OutcomeID. Keeping the ID and
// formula together prevents a valid proof about one outcome being relabeled
// as a proof about another.
type CompilerOutcomePredicate struct {
	OutcomeID string            `json:"outcome_id"`
	Predicate CompilerPredicate `json:"predicate"`
}

type ProofClaimKind string

const (
	ClaimReachability                ProofClaimKind = "reachability"
	ClaimUnreachability              ProofClaimKind = "unreachability"
	ClaimTotality                    ProofClaimKind = "totality"
	ClaimDisjointness                ProofClaimKind = "disjointness"
	ClaimExclusion                   ProofClaimKind = "exclusion"
	ClaimRealization                 ProofClaimKind = "realization"
	ClaimTestObservationCompleteness ProofClaimKind = "test-observation-completeness"
	ClaimScopeClosure                ProofClaimKind = "scope-closure"
)

// ProofClaim is the typed obligation from which central proof reconstructs
// the only accepted SMT query. Frontends cannot substitute an unrelated
// `(assert false)` query while retaining a valid replay record.
type ProofClaim struct {
	Kind        ProofClaimKind             `json:"kind"`
	Context     CompilerProofContext       `json:"context"`
	Scope       CompilerPredicate          `json:"scope"`
	Memberships []CompilerPredicate        `json:"memberships"`
	Outcomes    []CompilerOutcomePredicate `json:"outcomes"`
	LeftPass    *CompilerPredicate         `json:"left_pass,omitempty"`
	RightPass   *CompilerPredicate         `json:"right_pass,omitempty"`
}

// LabelPathEvidence grounds one spec-authored finite semantic label in an
// exact predicate/path of immutable compiler IR.
type LabelPathEvidence struct {
	ValueID                 string            `json:"value_id"`
	PredicateDigest         string            `json:"predicate_digest"`
	MembershipPredicate     CompilerPredicate `json:"membership_predicate"`
	CompilerNodeIDs         []string          `json:"compiler_node_ids"`
	Reachability            ProofResult       `json:"reachability"`
	ReachabilityProofDigest string            `json:"reachability_proof_digest"`
	ReachabilityProof       ReplayableProof   `json:"reachability_proof"`
	ConcreteWitness         *Literal          `json:"concrete_witness,omitempty"`
	WitnessDigest           string            `json:"witness_digest"`
	Provenance              Provenance        `json:"provenance"`
}

// ConstraintPathEvidence proves one spec-excluded operation assignment has
// no path inside the frozen compiler-IR scope.
type ConstraintPathEvidence struct {
	ConstraintID string          `json:"constraint_id"`
	Result       ProofResult     `json:"result"`
	ProofDigest  string          `json:"proof_digest"`
	Proof        ReplayableProof `json:"proof"`
	Provenance   Provenance      `json:"provenance"`
}

// OperationScopeEvidence identifies the exact concrete compiler-IR state
// space for an operation, including zero-argument operations that have no
// domain partition records.
type OperationScopeEvidence struct {
	OperationID          string            `json:"operation_id"`
	ScopePredicateDigest string            `json:"scope_predicate_digest"`
	ScopePredicate       CompilerPredicate `json:"scope_predicate"`
	Provenance           Provenance        `json:"provenance"`
}

// DomainPartitionEvidence proves that label predicates are total and
// pairwise-disjoint in the pre-constraint operation scope. A label may be
// refuted as unreachable only when the matching exclusions are also proved.
type DomainPartitionEvidence struct {
	OperationID             string                   `json:"operation_id"`
	DomainID                string                   `json:"domain_id"`
	ScopePredicateDigest    string                   `json:"scope_predicate_digest"`
	ScopePredicate          CompilerPredicate        `json:"scope_predicate"`
	Labels                  []LabelPathEvidence      `json:"labels"`
	Totality                ProofResult              `json:"totality"`
	TotalityProofDigest     string                   `json:"totality_proof_digest"`
	TotalityProof           ReplayableProof          `json:"totality_proof"`
	Disjointness            ProofResult              `json:"disjointness"`
	DisjointnessProofDigest string                   `json:"disjointness_proof_digest"`
	DisjointnessProof       ReplayableProof          `json:"disjointness_proof"`
	Exclusions              []ConstraintPathEvidence `json:"exclusions"`
	Provenance              Provenance               `json:"provenance"`
}

// BehaviorRealizationEvidence proves one entire semantic category assignment
// has exactly the modeled terminal/effects, not merely that one witness does.
// RealizationProof is UNSAT for category_conditions AND
// compiler_behavior != any OutcomeIDs member.
type BehaviorRealizationEvidence struct {
	BehaviorCaseID           string          `json:"behavior_case_id"`
	Behavior                 BehaviorRef     `json:"behavior"`
	OutcomeIDs               []string        `json:"outcome_ids"`
	CategoryPredicateDigests []string        `json:"category_predicate_digests"`
	RealizationProof         ReplayableProof `json:"realization_proof"`
	Provenance               Provenance      `json:"provenance"`
}

// CompilerEvidence binds the frontend projection to exact compiler bytes,
// invocation, emitted IR/harness, and finite abstraction proofs.
type CompilerEvidence struct {
	ID                      string                        `json:"id"`
	Method                  CompilerEvidenceMethod        `json:"method"`
	FormulaDerivationDigest string                        `json:"formula_derivation_digest"`
	SemanticGraph           *CompilerSemanticGraph        `json:"semantic_graph,omitempty"`
	Tool                    ToolRef                       `json:"tool"`
	Prover                  ToolRef                       `json:"prover"`
	SourceDigest            string                        `json:"source_digest"`
	WorkspaceTreeDigest     string                        `json:"workspace_tree_digest"`
	Argv                    []string                      `json:"argv"`
	EnvironmentDigest       string                        `json:"environment_digest"`
	IRKind                  CompilerIRKind                `json:"ir_kind"`
	EmittedIRDigest         string                        `json:"emitted_ir_digest"`
	HarnessDigest           string                        `json:"harness_digest"`
	TotalConstructs         int                           `json:"total_constructs"`
	TranslatedConstructs    int                           `json:"translated_constructs"`
	OperationScopes         []OperationScopeEvidence      `json:"operation_scopes"`
	Partitions              []DomainPartitionEvidence     `json:"partitions"`
	BehaviorProofs          []BehaviorRealizationEvidence `json:"behavior_proofs"`
	OutcomeClosures         []OutcomeClosureEvidence      `json:"outcome_closures"`
	Provenance              Provenance                    `json:"provenance"`
}

type OutcomeComplementKind string

const (
	OutcomeComplementReturn         OutcomeComplementKind = "other-return"
	OutcomeComplementRaise          OutcomeComplementKind = "other-raise"
	OutcomeComplementEffects        OutcomeComplementKind = "other-effect-vector"
	OutcomeComplementNontermination OutcomeComplementKind = "nontermination"
)

type OutcomeComplement struct {
	ID          string                   `json:"id"`
	Kind        OutcomeComplementKind    `json:"kind"`
	Description string                   `json:"description"`
	Predicate   CompilerOutcomePredicate `json:"predicate"`
}

// OutcomeClosureEvidence proves that declared outcomes plus explicit
// complement classes are a total, pairwise-disjoint partition of the exact
// observable boundary for one operation.
type OutcomeClosureEvidence struct {
	OperationID       string                     `json:"operation_id"`
	BoundaryDigest    string                     `json:"boundary_digest"`
	Declared          []CompilerOutcomePredicate `json:"declared"`
	Complements       []OutcomeComplement        `json:"complements"`
	Totality          ProofResult                `json:"totality"`
	TotalityProof     ReplayableProof            `json:"totality_proof"`
	Disjointness      ProofResult                `json:"disjointness"`
	DisjointnessProof ReplayableProof            `json:"disjointness_proof"`
	Provenance        Provenance                 `json:"provenance"`
}

// ConcreteDomainBinding is the compiler-derived exact source-language value
// used for one semantic label in exhaustive execution mode.
type ConcreteDomainBinding struct {
	OperationID string  `json:"operation_id"`
	DomainID    string  `json:"domain_id"`
	ValueID     string  `json:"value_id"`
	Value       Literal `json:"value"`
}

// RawEffectTrace is an ordered runtime fact. It deliberately has no semantic
// ID or provenance that a generated harness could use to self-classify.
type RawEffectTrace struct {
	Kind   EffectKind `json:"kind"`
	Target string     `json:"target"`
	Value  *Literal   `json:"value,omitempty"`
}

// RawOutcomeTrace contains only runtime facts emitted by an independently
// translated reference execution. Operation identity, semantic outcome ID,
// and provenance come only from frozen Hyperray inputs.
type RawOutcomeTrace struct {
	Kind          OutcomeKind      `json:"kind"`
	Value         *Literal         `json:"value,omitempty"`
	ExceptionType string           `json:"exception_type"`
	Message       string           `json:"message"`
	Effects       []RawEffectTrace `json:"effects"`
}

// ExecutionObservation is one fresh-process observation for one exact
// reachable semantic assignment.
type ExecutionObservation struct {
	Behavior          BehaviorRef        `json:"behavior"`
	Inputs            map[string]Literal `json:"inputs"`
	StepID            string             `json:"step_id"`
	RawOutcome        RawOutcomeTrace    `json:"raw_outcome"`
	OutcomeIDs        []string           `json:"outcome_ids"`
	ExitCode          int                `json:"exit_code"`
	Stdout            []byte             `json:"stdout"`
	StdoutDigest      string             `json:"stdout_digest"`
	StdoutTruncated   bool               `json:"stdout_truncated"`
	Stderr            []byte             `json:"stderr"`
	StderrDigest      string             `json:"stderr_digest"`
	StderrTruncated   bool               `json:"stderr_truncated"`
	SignalValue       []byte             `json:"signal_value"`
	SignalValueDigest string             `json:"signal_value_digest"`
	SignalTruncated   bool               `json:"signal_truncated"`
	Provenance        Provenance         `json:"provenance"`
}

type ExecutionRunEvidence struct {
	ID                string                 `json:"id"`
	StartedAtUTC      string                 `json:"started_at_utc"`
	Observations      []ExecutionObservation `json:"observations"`
	OrderDigest       string                 `json:"order_digest"`
	ObservationDigest string                 `json:"observation_digest"`
	FreshProcessCount int                    `json:"fresh_process_count"`
	Provenance        Provenance             `json:"provenance"`
}

// ExhaustiveExecutionEvidence is valid only for exact finite concrete
// groundings. It records every reachable assignment in fresh processes and
// repeats the full set in an independent order. Category abstractions still
// require CompilerEvidence realization proofs instead.
type ExhaustiveExecutionEvidence struct {
	ID                       string                   `json:"id"`
	Tool                     ToolRef                  `json:"tool"`
	SourceDigest             string                   `json:"source_digest"`
	WorkspaceTreeDigest      string                   `json:"workspace_tree_digest"`
	IRKind                   CompilerIRKind           `json:"ir_kind"`
	EmittedIRDigest          string                   `json:"emitted_ir_digest"`
	Harness                  []byte                   `json:"harness"`
	HarnessPath              string                   `json:"harness_path"`
	HarnessDigest            string                   `json:"harness_digest"`
	ExecutableDigest         string                   `json:"executable_digest"`
	Steps                    []ProbeStep              `json:"steps"`
	Argv                     []string                 `json:"argv"`
	WorkingDirectory         string                   `json:"working_directory"`
	Environment              []EnvironmentVariable    `json:"environment"`
	EnvironmentDigest        string                   `json:"environment_digest"`
	ClearEnvironment         bool                     `json:"clear_environment"`
	KillProcessGroup         bool                     `json:"kill_process_group"`
	TimeoutMillis            int64                    `json:"timeout_millis"`
	Groundings               []AssignmentGrounding    `json:"groundings"`
	CompleteAssignmentDigest string                   `json:"complete_assignment_digest"`
	Runs                     []ExecutionRunEvidence   `json:"runs"`
	Replay                   ExhaustiveReplayEvidence `json:"replay"`
	Complete                 bool                     `json:"complete"`
	Provenance               Provenance               `json:"provenance"`
}

type ExhaustiveReplayEvidence struct {
	CoreDigest       string                 `json:"core_digest"`
	StepsDigest      string                 `json:"steps_digest"`
	Runs             []ExecutionRunEvidence `json:"runs"`
	GeneratedOutputs []ProbeOutput          `json:"generated_outputs"`
	CleanupSteps     []ProbeStep            `json:"cleanup_steps"`
	CleanupPaths     []string               `json:"cleanup_paths"`
	CleanupDigest    string                 `json:"cleanup_digest"`
	Clean            bool                   `json:"clean"`
	Provenance       Provenance             `json:"provenance"`
}

// PassSignalKind declares how the verifier's pass result is read.
type PassSignalKind string

const (
	PassSignalExitCode PassSignalKind = "exit-code"
	PassSignalFile     PassSignalKind = "file"
)

// PassSignal is an exact observable verifier-pass declaration.
type PassSignal struct {
	Kind       PassSignalKind `json:"kind"`
	Path       string         `json:"path"`
	Expected   string         `json:"expected"`
	Provenance Provenance     `json:"provenance"`
}

// WorkspaceCommand binds one command and pass signal to a frozen workspace.
type WorkspaceCommand struct {
	ID                string                `json:"id"`
	WorkspaceID       string                `json:"workspace_id"`
	State             WorkspaceState        `json:"state"`
	TreeDigest        string                `json:"tree_digest"`
	Command           string                `json:"command"`
	WorkingDirectory  string                `json:"working_directory"`
	Environment       []EnvironmentVariable `json:"environment"`
	EnvironmentDigest string                `json:"environment_digest"`
	ClearEnvironment  bool                  `json:"clear_environment"`
	KillProcessGroup  bool                  `json:"kill_process_group"`
	TimeoutMillis     int64                 `json:"timeout_millis"`
	PassSignal        PassSignal            `json:"pass_signal"`
	ExpectedPass      bool                  `json:"expected_pass"`
	ObservedPass      bool                  `json:"observed_pass"`
	ExitCode          int                   `json:"exit_code"`
	StdoutDigest      string                `json:"stdout_digest"`
	StderrDigest      string                `json:"stderr_digest"`
	SignalValueDigest string                `json:"signal_value_digest"`
	Tools             []ToolRef             `json:"tools"`
	Provenance        Provenance            `json:"provenance"`
}

// EnvironmentModel is the explicit fifth independent model.
type EnvironmentModel struct {
	Artifact        ArtifactRef         `json:"artifact,omitempty"` // legacy decode alias for Configuration
	Configuration   ArtifactRef         `json:"configuration"`
	SourceArtifacts []ArtifactRef       `json:"source_artifacts"`
	Identity        string              `json:"identity"`
	ConfigDigest    string              `json:"config_digest"`
	Tools           []ToolRef           `json:"tools"`
	Commands        []WorkspaceCommand  `json:"commands"`
	Coverage        TranslationCoverage `json:"coverage"`
	Provenance      Provenance          `json:"provenance"`
}

// Task is the single finite proof input. Requirements describe Spec,
// CodeCases describe reference behavior, and TestSuite is the authoritative
// compiler-derived TestsPass predicate. Tests must exactly flatten the
// independently translated test artifact models.
type Task struct {
	ID               string           `json:"id"`
	Instruction      ArtifactRef      `json:"instruction"`
	InstructionModel InstructionModel `json:"instruction_model"`
	// Reference is the frozen reference solution. A benchmark instruction is
	// a problem statement with the rubric deliberately withheld, so a
	// requirement may anchor into the solution instead of the prompt.
	Reference ArtifactRef `json:"reference"`
	// Bridges from the finite model to the real system (proof-requirements.md
	// groups B and C): per-operation scope text and classifier command, and a
	// per-operation observer command for every string outcome label.
	Scopes         map[string]string            `json:"scopes"`
	Classifiers    map[string]string            `json:"classifiers"`
	Observers      map[string]map[string]string `json:"observers"`
	SpecAcceptance *SpecAcceptanceEvidence      `json:"spec_acceptance"`
	Environment    *EnvironmentModel            `json:"environment"`
	Spec           ArtifactRef                  `json:"spec"`
	SpecIRDigest   string                       `json:"spec_ir_digest"`
	Domains        []Domain                     `json:"domains"`
	Groundings     []AssignmentGrounding        `json:"groundings"`
	Constraints    []Constraint                 `json:"constraints"`
	Operations     []Operation                  `json:"operations"`
	Outcomes       []ObservableOutcome          `json:"outcomes"`
	Requirements   []RequirementCase            `json:"requirements"`
	Invariants     []Invariant                  `json:"invariants"`
	CodeCases      []BehaviorCase               `json:"code_cases"`
	Tests          []TestModel                  `json:"tests"`
	TestSuite      *TestSuiteModel              `json:"test_suite"`
	Artifacts      []ArtifactModel              `json:"artifacts"`
	Coverage       []TranslationCoverage        `json:"coverage"`
	Provenance     Provenance                   `json:"provenance"`
}

// ProofObligation identifies which exact set inclusion a witness refutes.
type ProofObligation string

const (
	ObligationReferenceCorrectness ProofObligation = "reference-within-spec"
	ObligationReferenceAcceptance  ProofObligation = "reference-accepted-by-tests"
	ObligationTestsSound           ProofObligation = "tests-pass-within-spec"
	ObligationTestsComplete        ProofObligation = "spec-within-tests-pass"
)

// Counterexample is a semantic witness produced by proof. It contains no
// source edit: a language frontend must materialize the witness against the
// exact frozen artifact before the executor can confirm it.
type Counterexample struct {
	ID               string           `json:"id"`
	Obligation       ProofObligation  `json:"obligation"`
	Conditions       Assignment       `json:"conditions"`
	OperationID      string           `json:"operation_id"`
	RequirementID    string           `json:"requirement_id"`
	Choices          []BehaviorChoice `json:"choices"`
	ObservedOutcomes []string         `json:"observed_outcomes"`
	ExpectedOutcomes []string         `json:"expected_outcomes"`
	TestPasses       bool             `json:"test_passes"`
	Provenance       Provenance       `json:"provenance"`
}

// MaterializationRequest gives a frontend the semantic witness together with
// the exact source request, translated model, and authoritative outcome
// vocabulary needed to realize it as byte edits.
type MaterializationRequest struct {
	Frontend       FrontendRequest `json:"frontend"`
	Task           *Task           `json:"task"`
	Model          ArtifactModel   `json:"model"`
	Counterexample Counterexample  `json:"counterexample"`
}

// ByteRangeReplacement replaces Source[start:end] using zero-based byte
// offsets, with EndByte exclusive. ExpectedBytes prevents an edit from being
// applied at a coincidentally valid range in changed source.
type ByteRangeReplacement struct {
	StartByte     int    `json:"start_byte"`
	EndByte       int    `json:"end_byte"`
	ExpectedBytes []byte `json:"expected_bytes"`
	Replacement   []byte `json:"replacement"`
}

type ProbeStepKind string

const (
	ProbeStepSetup   ProbeStepKind = "setup"
	ProbeStepRun     ProbeStepKind = "run"
	ProbeStepCleanup ProbeStepKind = "cleanup"
)

// ProbeStep is one direct executable invocation. Tool.Path is executed
// without a shell; Argv excludes argv[0].
type ProbeStep struct {
	ID                    string                `json:"id"`
	Kind                  ProbeStepKind         `json:"kind"`
	Tool                  ToolRef               `json:"tool"`
	GeneratedExecutableID string                `json:"generated_executable_id"`
	Argv                  []string              `json:"argv"`
	Stdin                 []byte                `json:"stdin"`
	StdinDigest           string                `json:"stdin_digest"`
	WorkingDirectory      string                `json:"working_directory"`
	Environment           []EnvironmentVariable `json:"environment"`
	EnvironmentDigest     string                `json:"environment_digest"`
	ClearEnvironment      bool                  `json:"clear_environment"`
	KillProcessGroup      bool                  `json:"kill_process_group"`
	TimeoutMillis         int64                 `json:"timeout_millis"`
	ExpectedExitCode      int                   `json:"expected_exit_code"`
	ExpectedStdoutDigest  string                `json:"expected_stdout_digest"`
	ExpectedStderrDigest  string                `json:"expected_stderr_digest"`
	ExpectedSignalDigest  string                `json:"expected_signal_digest"`
	SignalExtractor       ProbeSignalExtractor  `json:"signal_extractor"`
	Outputs               []ProbeOutput         `json:"outputs"`
	Provenance            Provenance            `json:"provenance"`
}

type ProbeSignalKind string

const (
	ProbeSignalNone             ProbeSignalKind = "none"
	ProbeSignalRawOutcomeStdout ProbeSignalKind = "raw-outcome-json-stdout"
	ProbeSignalRawOutcomeFile   ProbeSignalKind = "raw-outcome-json-file"
)

type ProbeSignalExtractor struct {
	Kind ProbeSignalKind `json:"kind"`
	Path string          `json:"path"`
}

// ProbeOutput is an exact file transition produced by a setup step. A later
// run step may execute it by GeneratedExecutableID without pretending the
// freshly generated binary was a pre-frozen ToolRef.
type ProbeOutput struct {
	ID            string     `json:"id"`
	Path          string     `json:"path"`
	ExistedBefore bool       `json:"existed_before"`
	BeforeDigest  string     `json:"before_digest"`
	AfterDigest   string     `json:"after_digest"`
	Executable    bool       `json:"executable"`
	Provenance    Provenance `json:"provenance"`
}

// ExpectedSemantics states what executing a materialized witness must show.
type ExpectedSemantics struct {
	Conditions      Assignment             `json:"conditions"`
	OperationID     string                 `json:"operation_id"`
	OutcomeIDs      []string               `json:"outcome_ids"`
	Choices         []BehaviorChoice       `json:"choices"`
	RuntimeOutcomes []RuntimeOutcomeChoice `json:"runtime_outcomes"`
	TestPasses      bool                   `json:"test_passes"`
}

// RuntimeOutcomeChoice is the concrete raw trace used to materialize a named
// exact outcome or the canonical OutcomeOther complement. RawOutcome.Kind may
// never be OutcomeOther; MappingOutcomeID is derived centrally.
type RuntimeOutcomeChoice struct {
	Behavior         BehaviorRef     `json:"behavior"`
	RawOutcome       RawOutcomeTrace `json:"raw_outcome"`
	MappingOutcomeID string          `json:"mapping_outcome_id"`
}

// EditPlan is an executable materialization anchored to immutable bytes.
type EditPlan struct {
	ID         string                 `json:"id"`
	WitnessID  string                 `json:"witness_id"`
	Artifact   ArtifactRef            `json:"artifact"`
	Edits      []ByteRangeReplacement `json:"edits"`
	Steps      []ProbeStep            `json:"steps"`
	Expected   ExpectedSemantics      `json:"expected"`
	Provenance Provenance             `json:"provenance"`
}
