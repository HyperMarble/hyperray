package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// WitnessKind names the three SAT witness classes from the frozen proof
// architecture. Reference acceptance T(C) is recorded separately because a
// failing clean reference is a valid NOT VERIFIED result, not a SAT witness.
type WitnessKind string

const (
	WitnessReference     WitnessKind = "reference"
	WitnessFalsePositive WitnessKind = "false-positive"
	WitnessFalseNegative WitnessKind = "false-negative"
)

// WitnessModelBindings bind execution to the exact independently compiled
// formal property, reference, test, environment, and proof result. The
// executor validates the SpecIR digest itself; the other canonical digests are
// retained for pipeline/certificate cross-checking against their typed IRs.
type WitnessModelBindings struct {
	SpecIRSHA256        string `json:"spec_ir_sha256"`
	ReferenceIRSHA256   string `json:"reference_ir_sha256"`
	TestIRSHA256        string `json:"test_ir_sha256"`
	EnvironmentIRSHA256 string `json:"environment_ir_sha256"`
	ProofResultSHA256   string `json:"proof_result_sha256"`
}

// FrozenWitnessContext is the complete immutable context shared by reference
// acceptance and a witness execution. Artifact groups stay distinct so a
// certificate cannot silently substitute test source for independently
// translated reference source (or vice versa).
type FrozenWitnessContext struct {
	Models               WitnessModelBindings     `json:"models"`
	Workspace            ProbeWorkspace           `json:"workspace"`
	ReferenceArtifacts   []semanticir.ArtifactRef `json:"reference_artifacts"`
	TestArtifacts        []semanticir.ArtifactRef `json:"test_artifacts"`
	EnvironmentArtifacts []semanticir.ArtifactRef `json:"environment_artifacts"`
	Tools                []semanticir.ToolRef     `json:"tools"`
	Execution            TaskEnvironment          `json:"execution"`
}

// ReferenceAcceptancePlan declares the exact clean C vector and the real
// frozen verifier invocation used to observe T(C).
type ReferenceAcceptancePlan struct {
	ID                     string                      `json:"id"`
	Context                FrozenWitnessContext        `json:"context"`
	ReferenceChoices       []semanticir.BehaviorChoice `json:"reference_choices"`
	ReferenceChoicesSHA256 string                      `json:"reference_choices_sha256"`
}

// WitnessPlan retains a complete proof witness and exactly one generic,
// frontend-produced execution mechanism. A reference witness uses a direct
// raw-outcome probe of the real C. False-positive/false-negative witnesses use
// digest-anchored edits which materialize the complete F atomically before the
// real verifier runs. The executor never authors either mechanism.
type WitnessPlan struct {
	ID        string                    `json:"id"`
	Kind      WitnessKind               `json:"kind"`
	Context   FrozenWitnessContext      `json:"context"`
	Witness   semanticir.Counterexample `json:"witness"`
	EditPlans []semanticir.EditPlan     `json:"edit_plans,omitempty"`
	Probe     *ProbePlan                `json:"probe,omitempty"`
}

// WitnessConfirmationRequest is the certificate-ready batch input. Baseline
// T(C) is mandatory even when Witnesses is empty.
type WitnessConfirmationRequest struct {
	ReferenceAcceptance ReferenceAcceptancePlan `json:"reference_acceptance"`
	Witnesses           []WitnessPlan           `json:"witnesses"`
}

// ReferenceAcceptanceEvidence is one fresh, isolated execution of the exact
// clean reference with the authoritative verifier and pass signal.
type ReferenceAcceptanceEvidence struct {
	Plan                     ReferenceAcceptancePlan `json:"plan"`
	PlanSHA256               string                  `json:"plan_sha256"`
	ContextSHA256            string                  `json:"context_sha256"`
	SourceBindings           []BindingEvidence       `json:"source_bindings"`
	ToolBindings             []BindingEvidence       `json:"tool_bindings"`
	BindingsVerifiedAfterRun bool                    `json:"bindings_verified_after_run"`
	Command                  CommandEvidence         `json:"command"`
	Isolation                *IsolationEvidence      `json:"isolation,omitempty"`
	ObservedPass             bool                    `json:"observed_pass"`
	Status                   Status                  `json:"status"`
	Blockers                 []Blocker               `json:"blockers,omitempty"`
}

// WitnessConfirmationEvidence binds a Confirmation to the full proof witness
// and all independent model/artifact/tool inputs. MaterializedChoices is the
// exact complete F represented by frontend edit plans. ObservedChoices is
// populated only when the real reference was freshly observed as raw traces.
type WitnessConfirmationEvidence struct {
	Plan                      WitnessPlan                 `json:"plan"`
	PlanSHA256                string                      `json:"plan_sha256"`
	ContextSHA256             string                      `json:"context_sha256"`
	WitnessSHA256             string                      `json:"witness_sha256"`
	ExpectedChoicesSHA256     string                      `json:"expected_choices_sha256"`
	MaterializedChoices       []semanticir.BehaviorChoice `json:"materialized_choices,omitempty"`
	MaterializedChoicesSHA256 string                      `json:"materialized_choices_sha256,omitempty"`
	ObservedChoices           []semanticir.BehaviorChoice `json:"observed_choices,omitempty"`
	ObservedChoicesSHA256     string                      `json:"observed_choices_sha256,omitempty"`
	BaselineVector            bool                        `json:"baseline_vector,omitempty"`
	SourceBindings            []BindingEvidence           `json:"source_bindings"`
	ToolBindings              []BindingEvidence           `json:"tool_bindings"`
	BindingsVerifiedAfterRun  bool                        `json:"bindings_verified_after_run"`
	ModelExecutionMatch       bool                        `json:"model_execution_match"`
	Error                     string                      `json:"error,omitempty"`
}

type preparedWitnessContext struct {
	value   FrozenWitnessContext
	root    string
	task    TaskEnvironment
	sources []preparedBinding
	tools   []preparedBinding
}

type preparedWitnessPlan struct {
	plan           WitnessPlan
	context        preparedWitnessContext
	edits          []preparedPlan
	probe          *preparedProbe
	baselineVector bool
}

// ConfirmWitnessesIsolated executes the corrected frozen architecture. It
// performs exactly one clean T(C) run in a disposable copy, then executes each
// proof witness in its own new disposable copy. All plans are preflighted
// before any command runs. A semantic/pass disagreement is a model defect and
// therefore PROOF BLOCKED; T(C)=false is retained as valid NOT VERIFIED
// evidence.
func ConfirmWitnessesIsolated(ctx context.Context, task *semanticir.Task, request WitnessConfirmationRequest) (report Report) {
	report.Status = StatusProofBlocked
	report.Vacuous = len(request.Witnesses) == 0
	report.ExpectedConfirmations = len(request.Witnesses)
	if ctx == nil {
		report.Blockers = append(report.Blockers, Blocker{Stage: "preflight", Code: "nil-context", Detail: "execution context is nil"})
		return report
	}
	acceptanceContext, blocker := prepareWitnessContext(task, request.ReferenceAcceptance.Context)
	if blocker != nil {
		report.Blockers = append(report.Blockers, *blocker)
		return report
	}
	if err := validateReferenceAcceptancePlan(task, request.ReferenceAcceptance); err != nil {
		report.Blockers = append(report.Blockers, Blocker{Stage: "preflight", Code: "invalid-reference-acceptance", Detail: err.Error()})
		return report
	}
	prepared := make([]preparedWitnessPlan, 0, len(request.Witnesses))
	seen := make(map[string]bool, len(request.Witnesses))
	for _, plan := range request.Witnesses {
		if seen[plan.ID] {
			report.Blockers = append(report.Blockers, Blocker{Stage: "preflight", PlanID: plan.ID, WitnessID: plan.Witness.ID, Code: "duplicate-witness-plan", Detail: "witness plan IDs must be unique"})
			return report
		}
		seen[plan.ID] = true
		candidate, candidateBlocker := prepareWitnessPlan(task, request.ReferenceAcceptance, plan)
		if candidateBlocker != nil {
			report.Blockers = append(report.Blockers, *candidateBlocker)
			return report
		}
		prepared = append(prepared, candidate)
	}

	acceptance := runReferenceAcceptance(ctx, request.ReferenceAcceptance, acceptanceContext)
	report.ReferenceAcceptance = &acceptance
	report.Baseline = acceptance.Command
	report.BaselineIsolation = acceptance.Isolation
	report.Blockers = append(report.Blockers, acceptance.Blockers...)
	if acceptance.Status == StatusProofBlocked {
		finalizeWitnessReport(&report)
		return report
	}

	for _, candidate := range prepared {
		confirmation := executeWitnessPlan(ctx, candidate, acceptance)
		report.Confirmations = append(report.Confirmations, confirmation)
		report.Blockers = append(report.Blockers, confirmation.Blockers...)
	}
	finalizeWitnessReport(&report)
	return report
}

func prepareWitnessPlan(task *semanticir.Task, acceptance ReferenceAcceptancePlan, plan WitnessPlan) (preparedWitnessPlan, *Blocker) {
	block := func(code, detail string) (preparedWitnessPlan, *Blocker) {
		return preparedWitnessPlan{}, &Blocker{Stage: "preflight", PlanID: plan.ID, WitnessID: plan.Witness.ID, Code: code, Detail: detail}
	}
	if plan.ID == "" || plan.Witness.ID == "" {
		return block("unbound-witness", "witness plan and semantic witness IDs must be non-empty")
	}
	if !reflect.DeepEqual(plan.Context, acceptance.Context) {
		return block("detached-witness-context", "witness context differs from the mandatory T(C) context")
	}
	if diagnostics := semanticir.ValidateCounterexample(task, plan.Witness); semanticir.HasErrors(diagnostics) {
		return block("invalid-counterexample", firstDiagnostic(diagnostics))
	}
	if err := validateWitnessKind(plan.Kind, plan.Witness); err != nil {
		return block("wrong-witness-obligation", err.Error())
	}
	preparedContext, contextBlocker := prepareWitnessContext(task, plan.Context)
	if contextBlocker != nil {
		contextBlocker.PlanID, contextBlocker.WitnessID = plan.ID, plan.Witness.ID
		return preparedWitnessPlan{}, contextBlocker
	}
	prepared := preparedWitnessPlan{plan: plan, context: preparedContext}
	switch plan.Kind {
	case WitnessReference:
		if plan.Probe == nil || len(plan.EditPlans) != 0 {
			return block("invalid-reference-mechanism", "reference witness requires exactly one frontend-generated direct probe and no source edits")
		}
		if !reflect.DeepEqual(plan.Probe.Witness, plan.Witness) || plan.Probe.WitnessID != plan.Witness.ID || !reflect.DeepEqual(plan.Probe.Workspace, plan.Context.Workspace) {
			return block("detached-reference-probe", "reference probe differs from the full witness or frozen workspace")
		}
		probe, probeBlocker := prepareProbePlan(*plan.Probe)
		if probeBlocker != nil {
			probeBlocker.PlanID, probeBlocker.WitnessID = plan.ID, plan.Witness.ID
			return preparedWitnessPlan{}, probeBlocker
		}
		prepared.probe = &probe
	case WitnessFalsePositive, WitnessFalseNegative:
		if plan.Probe != nil {
			return block("invalid-materialization-mechanism", "false-positive/false-negative witness may not use a reference probe")
		}
		if len(plan.EditPlans) == 0 {
			if !sameSemanticChoices(plan.Witness.Choices, acceptance.ReferenceChoices) {
				return block("missing-materialization", "non-baseline false-positive/false-negative witness has no frontend edit plan")
			}
			prepared.baselineVector = true
			return prepared, nil
		}
		for _, edit := range plan.EditPlans {
			if edit.WitnessID != plan.Witness.ID || !expectedPreservesWitness(edit.Expected, plan.Witness) {
				return block("detached-edit-semantics", fmt.Sprintf("edit plan %q does not preserve the complete proof witness", edit.ID))
			}
			if !artifactInRefs(edit.Artifact, plan.Context.ReferenceArtifacts) {
				return block("unbound-edit-artifact", fmt.Sprintf("edit plan %q is not bound to an independently translated reference artifact", edit.ID))
			}
		}
		edits, blockers := preparePlans(preparedContext.task, plan.EditPlans)
		if len(blockers) != 0 {
			return preparedWitnessPlan{}, &blockers[0]
		}
		groups := groupPlans(edits)
		if len(groups) != 1 || len(groups[0]) != len(edits) {
			return block("non-atomic-witness", "all edit plans for one complete behavior vector must share one witness ID")
		}
		prepared.edits = edits
	default:
		return block("wrong-witness-obligation", fmt.Sprintf("unsupported witness kind %q", plan.Kind))
	}
	return prepared, nil
}

func executeWitnessPlan(ctx context.Context, prepared preparedWitnessPlan, acceptance ReferenceAcceptanceEvidence) Confirmation {
	var confirmation Confirmation
	if prepared.baselineVector {
		observed := acceptance.ObservedPass
		confirmation = Confirmation{
			WitnessID: prepared.plan.Witness.ID, Mode: ConfirmationModeBaselineVector,
			Status: StatusConfirmed, ExpectedTestPasses: prepared.plan.Witness.TestPasses,
			ObservedTestPasses: &observed, Command: acceptance.Command, Isolation: acceptance.Isolation,
		}
		if observed != prepared.plan.Witness.TestPasses {
			confirmation.Status = StatusProofBlocked
			confirmation.Blockers = append(confirmation.Blockers, Blocker{
				Stage: "confirmation", PlanID: prepared.plan.ID, WitnessID: prepared.plan.Witness.ID,
				Code: "model-execution-mismatch", Detail: fmt.Sprintf("frozen T(C) observed %t, but the baseline-equal semantic witness requires %t", observed, prepared.plan.Witness.TestPasses),
			})
		}
	} else if prepared.probe != nil {
		confirmation, _ = confirmProbe(ctx, *prepared.probe)
	} else {
		confirmation, _ = confirmIsolatedGroup(ctx, prepared.context.task, prepared.edits)
	}
	evidence := &WitnessConfirmationEvidence{
		Plan: planClone(prepared.plan), PlanSHA256: mustProbeDigest(prepared.plan),
		ContextSHA256: mustProbeDigest(prepared.plan.Context), WitnessSHA256: mustProbeDigest(prepared.plan.Witness),
		ExpectedChoicesSHA256: mustProbeDigest(prepared.plan.Witness.Choices),
		SourceBindings:        bindingEvidence(prepared.context.sources), ToolBindings: bindingEvidence(prepared.context.tools),
		BaselineVector: prepared.baselineVector,
	}
	if prepared.probe != nil && confirmation.Probe != nil {
		for _, observed := range confirmation.Probe.Derived {
			evidence.ObservedChoices = append(evidence.ObservedChoices, semanticir.BehaviorChoice{Behavior: observed.Behavior, OutcomeID: observed.ClassifiedOutcomeID})
		}
		evidence.ObservedChoicesSHA256 = mustProbeDigest(evidence.ObservedChoices)
	} else {
		evidence.MaterializedChoices = append([]semanticir.BehaviorChoice(nil), prepared.plan.Witness.Choices...)
		evidence.MaterializedChoicesSHA256 = mustProbeDigest(evidence.MaterializedChoices)
	}
	if err := verifyWitnessContextBindings(prepared.context); err != nil {
		evidence.Error = err.Error()
		confirmation.Blockers = append(confirmation.Blockers, Blocker{Stage: "cleanup", PlanID: prepared.plan.ID, WitnessID: prepared.plan.Witness.ID, Code: "frozen-binding-changed", Detail: err.Error()})
		confirmation.Status = StatusProofBlocked
	} else {
		evidence.BindingsVerifiedAfterRun = true
	}
	evidence.ModelExecutionMatch = confirmation.Status == StatusConfirmed
	confirmation.WitnessExecution = evidence
	return confirmation
}

func runReferenceAcceptance(ctx context.Context, plan ReferenceAcceptancePlan, prepared preparedWitnessContext) ReferenceAcceptanceEvidence {
	evidence := ReferenceAcceptanceEvidence{
		Plan: plan, PlanSHA256: mustProbeDigest(plan), ContextSHA256: mustProbeDigest(plan.Context),
		SourceBindings: bindingEvidence(prepared.sources), ToolBindings: bindingEvidence(prepared.tools), Status: StatusProofBlocked,
	}
	command, isolation, blocker := runIsolatedBaseline(ctx, prepared.task)
	evidence.Command, evidence.Isolation, evidence.ObservedPass = command, isolation, command.Passed
	if blocker != nil {
		evidence.Blockers = append(evidence.Blockers, *blocker)
		return evidence
	}
	if err := verifyWitnessContextBindings(prepared); err != nil {
		evidence.Blockers = append(evidence.Blockers, Blocker{Stage: "cleanup", PlanID: plan.ID, Code: "frozen-binding-changed", Detail: err.Error()})
		return evidence
	}
	evidence.BindingsVerifiedAfterRun = true
	if command.Passed {
		evidence.Status = StatusConfirmed
	} else {
		evidence.Status = StatusNotConfirmed
	}
	return evidence
}

func prepareWitnessContext(task *semanticir.Task, value FrozenWitnessContext) (preparedWitnessContext, *Blocker) {
	block := func(code, detail string) (preparedWitnessContext, *Blocker) {
		return preparedWitnessContext{}, &Blocker{Stage: "preflight", Code: code, Detail: detail}
	}
	if task == nil {
		return block("nil-semantic-task", "compiled semantic task is nil")
	}
	if semanticir.HasErrors(semanticir.ValidateSpecIRDigest(task)) || value.Models.SpecIRSHA256 != task.SpecIRDigest {
		return block("stale-formal-property", "witness is not bound to the canonical compiled Spec IR")
	}
	if !validWitnessModelBindings(value.Models) {
		return block("invalid-model-bindings", "reference, test, environment, proof, or Spec IR digest is missing")
	}
	if semanticir.HasErrors(semanticir.ValidateReferenceIR(task)) || semanticir.HasErrors(semanticir.ValidateTestIR(task)) || semanticir.HasErrors(semanticir.ValidateEnvironmentIR(task)) {
		return block("invalid-model-bindings", "reference, test, or environment IR is incomplete")
	}
	referenceDigest, referenceErr := semanticir.CanonicalReferenceIRDigest(task)
	testDigest, testErr := semanticir.CanonicalTestIRDigest(task)
	environmentDigest, environmentErr := semanticir.CanonicalEnvironmentIRDigest(task)
	if referenceErr != nil || testErr != nil || environmentErr != nil || referenceDigest != value.Models.ReferenceIRSHA256 || testDigest != value.Models.TestIRSHA256 || environmentDigest != value.Models.EnvironmentIRSHA256 {
		return block("stale-model-bindings", "witness reference, test, or environment IR digest differs from the compiled task")
	}
	resolvedWork, resolvedRoot, taskBlocker := validateTask(value.Execution)
	if taskBlocker != nil {
		return preparedWitnessContext{}, taskBlocker
	}
	if !value.Execution.ExactEnvironment {
		return block("ambient-environment", "authoritative witness execution must clear the ambient environment")
	}
	if !filepath.IsAbs(value.Execution.Command[0]) {
		return block("unfrozen-command", "authoritative verifier argv[0] must be an absolute frozen tool path")
	}
	value.Execution.WorkDir, value.Execution.WorkspaceRoot = resolvedWork, resolvedRoot
	if value.Workspace.ID == "" || value.Workspace.State != semanticir.WorkspaceSolutionNewTests || value.Workspace.TreeSHA256 != value.Execution.WorkspaceSHA256 || !validDigest(value.Workspace.TreeSHA256) {
		return block("invalid-witness-workspace", "witness context does not bind the solution+new-tests workspace digest")
	}
	workspaceRoot, err := filepath.EvalSymlinks(value.Workspace.Root)
	if err != nil || filepath.Clean(workspaceRoot) != resolvedRoot {
		return block("detached-witness-workspace", "witness workspace differs from the execution workspace")
	}
	observedWorkspace, err := WorkspaceDigest(resolvedRoot)
	if err != nil || observedWorkspace != value.Workspace.TreeSHA256 {
		return block("stale-workspace", fmt.Sprintf("workspace digest is %s, want %s", observedWorkspace, value.Workspace.TreeSHA256))
	}
	if len(value.ReferenceArtifacts) == 0 || len(value.TestArtifacts) == 0 || len(value.EnvironmentArtifacts) == 0 {
		return block("missing-artifact-bindings", "reference, test, and environment artifact bindings are all mandatory")
	}
	allRefs := append(append(append([]semanticir.ArtifactRef(nil), value.ReferenceArtifacts...), value.TestArtifacts...), value.EnvironmentArtifacts...)
	sources, err := prepareWitnessArtifacts(resolvedRoot, allRefs)
	if err != nil {
		return block("invalid-artifact-binding", err.Error())
	}
	tools, err := prepareWitnessTools(value.Tools)
	if err != nil {
		return block("invalid-tool-binding", err.Error())
	}
	commandPath, err := filepath.EvalSymlinks(value.Execution.Command[0])
	if err != nil {
		return block("unfrozen-command", fmt.Sprintf("resolve verifier command: %v", err))
	}
	commandBound := false
	for _, tool := range tools {
		commandBound = commandBound || samePath(commandPath, tool.path)
	}
	if !commandBound {
		return block("unfrozen-command", "verifier argv[0] is not one of the frozen tool identities")
	}
	return preparedWitnessContext{value: value, root: resolvedRoot, task: value.Execution, sources: sources, tools: tools}, nil
}

func prepareWitnessArtifacts(root string, refs []semanticir.ArtifactRef) ([]preparedBinding, error) {
	result := make([]preparedBinding, 0, len(refs))
	ids, paths := map[string]bool{}, map[string]bool{}
	for _, ref := range refs {
		if ref.ID == "" || ref.Path == "" || !validDigest(ref.Digest) || ids[ref.ID] {
			return nil, fmt.Errorf("artifact binding has an empty/duplicate ID, path, or digest")
		}
		path, err := resolveProbeExisting(root, ref.Path, false)
		if err != nil || paths[path] {
			return nil, fmt.Errorf("artifact %q path is outside the frozen workspace or duplicated", ref.ID)
		}
		body, err := os.ReadFile(path)
		if err != nil || digestBytes(body) != ref.Digest {
			return nil, fmt.Errorf("artifact %q differs from its frozen digest", ref.ID)
		}
		ids[ref.ID], paths[path] = true, true
		result = append(result, preparedBinding{ref: ref, path: path, digest: ref.Digest})
	}
	return result, nil
}

func prepareWitnessTools(refs []semanticir.ToolRef) ([]preparedBinding, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("no frozen tools are declared")
	}
	result := make([]preparedBinding, 0, len(refs))
	names, paths := map[string]bool{}, map[string]bool{}
	for _, ref := range refs {
		if ref.Name == "" || ref.Version == "" || !filepath.IsAbs(ref.Path) || !validDigest(ref.Digest) || names[ref.Name] {
			return nil, fmt.Errorf("tool binding has an empty/duplicate name, version, absolute path, or digest")
		}
		path, err := filepath.EvalSymlinks(ref.Path)
		if err != nil || paths[path] {
			return nil, fmt.Errorf("tool %q path cannot be resolved uniquely", ref.Name)
		}
		info, statErr := os.Lstat(path)
		body, readErr := os.ReadFile(path)
		if statErr != nil || readErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || digestBytes(body) != ref.Digest {
			return nil, fmt.Errorf("tool %q differs from its frozen executable", ref.Name)
		}
		names[ref.Name], paths[path] = true, true
		result = append(result, preparedBinding{tool: ref, path: filepath.Clean(path), digest: ref.Digest, version: ref.Version})
	}
	return result, nil
}

func verifyWitnessContextBindings(prepared preparedWitnessContext) error {
	for _, source := range prepared.sources {
		body, err := os.ReadFile(source.path)
		if err != nil || digestBytes(body) != source.digest {
			return fmt.Errorf("artifact %q changed during isolated execution", source.ref.ID)
		}
	}
	return verifyPreparedProbeTools(prepared.tools)
}

func validateReferenceAcceptancePlan(task *semanticir.Task, plan ReferenceAcceptancePlan) error {
	if plan.ID == "" || !validDigest(plan.ReferenceChoicesSHA256) || plan.ReferenceChoicesSHA256 != mustProbeDigest(plan.ReferenceChoices) {
		return fmt.Errorf("reference acceptance has an incomplete ID or vector digest")
	}
	want, err := exactReferenceChoices(task)
	if err != nil {
		return err
	}
	if !sameSemanticChoices(plan.ReferenceChoices, want) {
		return fmt.Errorf("reference acceptance vector differs from independently translated C")
	}
	return nil
}

func exactReferenceChoices(task *semanticir.Task) ([]semanticir.BehaviorChoice, error) {
	choices := make([]semanticir.BehaviorChoice, 0, len(task.CodeCases))
	for _, behaviorCase := range task.CodeCases {
		if len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OperationID == "" {
			return nil, fmt.Errorf("reference case %q is not one exact concrete behavior", behaviorCase.ID)
		}
		choices = append(choices, semanticir.BehaviorChoice{Behavior: semanticir.BehaviorRef{
			OperationID: behaviorCase.OperationID, Conditions: behaviorCase.Conditions, Inputs: behaviorCase.Inputs, Provenance: behaviorCase.Provenance,
		}, OutcomeID: behaviorCase.OutcomeIDs[0]})
	}
	sort.Slice(choices, func(i, j int) bool { return baselineChoiceKey(choices[i]) < baselineChoiceKey(choices[j]) })
	if len(choices) == 0 {
		return nil, fmt.Errorf("reference IR has no exact behavior choices")
	}
	return choices, nil
}

func validateWitnessKind(kind WitnessKind, witness semanticir.Counterexample) error {
	switch kind {
	case WitnessReference:
		if witness.Obligation != semanticir.ObligationReferenceCorrectness {
			return fmt.Errorf("reference witness has obligation %q", witness.Obligation)
		}
	case WitnessFalsePositive:
		if witness.Obligation != semanticir.ObligationTestsSound || !witness.TestPasses {
			return fmt.Errorf("false-positive witness must satisfy T(F)")
		}
	case WitnessFalseNegative:
		if witness.Obligation != semanticir.ObligationTestsComplete || witness.TestPasses {
			return fmt.Errorf("false-negative witness must violate T(F)")
		}
	default:
		return fmt.Errorf("unsupported witness kind %q", kind)
	}
	return nil
}

func expectedPreservesWitness(expected semanticir.ExpectedSemantics, witness semanticir.Counterexample) bool {
	return reflect.DeepEqual(expected.Conditions, witness.Conditions) && expected.OperationID == witness.OperationID &&
		reflect.DeepEqual(expected.OutcomeIDs, witness.ObservedOutcomes) && reflect.DeepEqual(expected.Choices, witness.Choices) &&
		expected.TestPasses == witness.TestPasses
}

func artifactInRefs(artifact semanticir.ArtifactRef, refs []semanticir.ArtifactRef) bool {
	for _, ref := range refs {
		if reflect.DeepEqual(ref, artifact) {
			return true
		}
	}
	return false
}

func validWitnessModelBindings(bindings WitnessModelBindings) bool {
	return validDigest(bindings.SpecIRSHA256) && validDigest(bindings.ReferenceIRSHA256) && validDigest(bindings.TestIRSHA256) &&
		validDigest(bindings.EnvironmentIRSHA256) && validDigest(bindings.ProofResultSHA256)
}

func bindingEvidence(bindings []preparedBinding) []BindingEvidence {
	result := make([]BindingEvidence, 0, len(bindings))
	for _, binding := range bindings {
		if binding.ref.ID != "" {
			result = append(result, BindingEvidence{ID: binding.ref.ID, Path: binding.path, ExpectedSHA256: binding.digest, ObservedSHA256: binding.digest, Verified: true})
		} else {
			result = append(result, BindingEvidence{ID: binding.tool.Name, Path: binding.path, ExpectedSHA256: binding.digest, ObservedSHA256: binding.digest, Version: binding.version, Verified: true})
		}
	}
	return result
}

func firstDiagnostic(diagnostics []semanticir.Diagnostic) string {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == semanticir.SeverityError {
			return string(diagnostic.Code) + ": " + diagnostic.Message
		}
	}
	return "counterexample validation failed"
}

func planClone(plan WitnessPlan) WitnessPlan { return plan }

func finalizeWitnessReport(report *Report) {
	report.ConfirmationSHA256 = report.ConfirmationSHA256[:0]
	for _, confirmation := range report.Confirmations {
		report.ConfirmationSHA256 = append(report.ConfirmationSHA256, mustProbeDigest(confirmation))
	}
	switch {
	case len(report.Blockers) != 0 || report.ReferenceAcceptance == nil || report.ReferenceAcceptance.Status == StatusProofBlocked:
		report.Status = StatusProofBlocked
	case report.ReferenceAcceptance.Status == StatusNotConfirmed:
		report.Status = StatusNotConfirmed
	default:
		report.Status = StatusConfirmed
		for _, confirmation := range report.Confirmations {
			if confirmation.Status == StatusProofBlocked {
				report.Status = StatusProofBlocked
				break
			}
			if confirmation.Status != StatusConfirmed {
				report.Status = StatusNotConfirmed
			}
		}
	}
	projection := *report
	projection.EvidenceSHA256 = ""
	report.EvidenceSHA256 = mustProbeDigest(projection)
}

// ValidateReferenceAcceptance validates a retained clean T(C) execution
// without consulting mutable files. A clean rejection is valid evidence and
// therefore accepts StatusNotConfirmed as well as StatusConfirmed.
func ValidateReferenceAcceptance(evidence ReferenceAcceptanceEvidence) error {
	if evidence.Status != StatusConfirmed && evidence.Status != StatusNotConfirmed {
		return fmt.Errorf("reference acceptance is blocked or has an unknown status")
	}
	if len(evidence.Blockers) != 0 || evidence.PlanSHA256 != mustProbeDigest(evidence.Plan) || evidence.ContextSHA256 != mustProbeDigest(evidence.Plan.Context) || !evidence.BindingsVerifiedAfterRun || evidence.ObservedPass != evidence.Command.Passed {
		return fmt.Errorf("reference acceptance identity, bindings, or pass result is inconsistent")
	}
	if (evidence.Status == StatusConfirmed) != evidence.ObservedPass {
		return fmt.Errorf("reference acceptance status differs from T(C)")
	}
	if err := validateCommandEvidenceIntegrity(evidence.Command); err != nil {
		return err
	}
	if evidence.Isolation == nil || !validGenericIsolation(*evidence.Isolation) {
		return fmt.Errorf("reference acceptance lacks fresh isolated execution evidence")
	}
	if err := validateRetainedBindings(evidence.Plan.Context, evidence.SourceBindings, evidence.ToolBindings); err != nil {
		return err
	}
	return nil
}

// ValidateReferenceWitnessConfirmation validates a confirmed direct
// observation of the independently translated real reference C.
func ValidateReferenceWitnessConfirmation(confirmation Confirmation) error {
	return validateTypedWitnessConfirmation(confirmation, WitnessReference)
}

// ValidateFalsePositiveWitnessConfirmation validates a complete materialized
// F for which the real verifier freshly observed T(F)=true.
func ValidateFalsePositiveWitnessConfirmation(confirmation Confirmation) error {
	return validateTypedWitnessConfirmation(confirmation, WitnessFalsePositive)
}

// ValidateFalseNegativeWitnessConfirmation validates a complete permitted F
// for which the real verifier freshly observed T(F)=false.
func ValidateFalseNegativeWitnessConfirmation(confirmation Confirmation) error {
	return validateTypedWitnessConfirmation(confirmation, WitnessFalseNegative)
}

func validateTypedWitnessConfirmation(confirmation Confirmation, kind WitnessKind) error {
	if confirmation.Status != StatusConfirmed || confirmation.WitnessExecution == nil || len(confirmation.Blockers) != 0 {
		return fmt.Errorf("typed witness confirmation is blocked or incomplete")
	}
	evidence := confirmation.WitnessExecution
	plan := evidence.Plan
	if plan.Kind != kind || plan.Witness.ID != confirmation.WitnessID || evidence.PlanSHA256 != mustProbeDigest(plan) || evidence.ContextSHA256 != mustProbeDigest(plan.Context) || evidence.WitnessSHA256 != mustProbeDigest(plan.Witness) || evidence.ExpectedChoicesSHA256 != mustProbeDigest(plan.Witness.Choices) || !evidence.BindingsVerifiedAfterRun || !evidence.ModelExecutionMatch || evidence.Error != "" {
		return fmt.Errorf("typed witness identity, model, or binding evidence is inconsistent")
	}
	if err := validateWitnessKind(kind, plan.Witness); err != nil {
		return err
	}
	if !validWitnessModelBindings(plan.Context.Models) {
		return fmt.Errorf("typed witness has incomplete independent model digests")
	}
	if err := validateRetainedBindings(plan.Context, evidence.SourceBindings, evidence.ToolBindings); err != nil {
		return err
	}
	switch kind {
	case WitnessReference:
		if plan.Probe == nil || len(plan.EditPlans) != 0 || len(evidence.MaterializedChoices) != 0 || !reflect.DeepEqual(evidence.ObservedChoices, plan.Witness.Choices) || evidence.ObservedChoicesSHA256 != mustProbeDigest(evidence.ObservedChoices) {
			return fmt.Errorf("reference confirmation does not retain the exact observed C vector")
		}
		return ValidateProbeConfirmation(confirmation)
	case WitnessFalsePositive, WitnessFalseNegative:
		if plan.Probe != nil || len(evidence.ObservedChoices) != 0 || !reflect.DeepEqual(evidence.MaterializedChoices, plan.Witness.Choices) || evidence.MaterializedChoicesSHA256 != mustProbeDigest(evidence.MaterializedChoices) {
			return fmt.Errorf("candidate confirmation does not retain the exact materialized F vector")
		}
		if evidence.BaselineVector {
			if len(plan.EditPlans) != 0 || confirmation.Mode != ConfirmationModeBaselineVector || confirmation.ObservedTestPasses == nil || *confirmation.ObservedTestPasses != plan.Witness.TestPasses || confirmation.Isolation == nil || !validGenericIsolation(*confirmation.Isolation) {
				return fmt.Errorf("baseline-equal candidate does not retain exact fresh T(C) evidence")
			}
			return validateCommandEvidenceIntegrity(confirmation.Command)
		}
		if len(plan.EditPlans) == 0 {
			return fmt.Errorf("non-baseline candidate has no frontend materialization")
		}
		if err := ValidateEditConfirmation(confirmation); err != nil {
			return err
		}
	}
	return nil
}

// ValidateWitnessReport checks the stable aggregate, including mandatory
// T(C), exact cardinality/digests, compiled IR bindings, and the pure validator
// for every confirmed witness mode.
func ValidateWitnessReport(task *semanticir.Task, report Report) error {
	if task == nil || report.ReferenceAcceptance == nil {
		return fmt.Errorf("witness report omits the semantic task or mandatory T(C) evidence")
	}
	if report.ExpectedConfirmations != len(report.Confirmations) || report.Vacuous != (len(report.Confirmations) == 0) || len(report.ConfirmationSHA256) != len(report.Confirmations) {
		return fmt.Errorf("witness report confirmation cardinality is inconsistent")
	}
	if err := validateReferenceAcceptancePlan(task, report.ReferenceAcceptance.Plan); err != nil {
		return err
	}
	if err := ValidateReferenceAcceptance(*report.ReferenceAcceptance); err != nil {
		return err
	}
	if !reflect.DeepEqual(report.Baseline, report.ReferenceAcceptance.Command) || !reflect.DeepEqual(report.BaselineIsolation, report.ReferenceAcceptance.Isolation) {
		return fmt.Errorf("witness report baseline differs from typed T(C) evidence")
	}
	for index, confirmation := range report.Confirmations {
		if report.ConfirmationSHA256[index] != mustProbeDigest(confirmation) {
			return fmt.Errorf("witness confirmation digest %d differs", index)
		}
		if confirmation.WitnessExecution == nil {
			return fmt.Errorf("witness confirmation %d has no typed execution evidence", index)
		}
		if !reflect.DeepEqual(confirmation.WitnessExecution.Plan.Context, report.ReferenceAcceptance.Plan.Context) {
			return fmt.Errorf("witness confirmation %d is detached from T(C) context", index)
		}
		if confirmation.WitnessExecution.BaselineVector && (!reflect.DeepEqual(confirmation.Command, report.ReferenceAcceptance.Command) || !reflect.DeepEqual(confirmation.Isolation, report.ReferenceAcceptance.Isolation)) {
			return fmt.Errorf("baseline-equal witness confirmation %d differs from fresh T(C) execution", index)
		}
		var err error
		switch confirmation.WitnessExecution.Plan.Kind {
		case WitnessReference:
			err = ValidateReferenceWitnessConfirmation(confirmation)
		case WitnessFalsePositive:
			err = ValidateFalsePositiveWitnessConfirmation(confirmation)
		case WitnessFalseNegative:
			err = ValidateFalseNegativeWitnessConfirmation(confirmation)
		default:
			err = fmt.Errorf("unsupported typed witness kind")
		}
		if err != nil {
			return fmt.Errorf("witness confirmation %d: %w", index, err)
		}
	}
	projection := report
	projection.EvidenceSHA256 = ""
	if !validDigest(report.EvidenceSHA256) || report.EvidenceSHA256 != mustProbeDigest(projection) {
		return fmt.Errorf("witness report aggregate digest differs")
	}
	wantStatus := StatusConfirmed
	if report.ReferenceAcceptance.Status == StatusNotConfirmed {
		wantStatus = StatusNotConfirmed
	}
	if len(report.Blockers) != 0 {
		wantStatus = StatusProofBlocked
	}
	if report.Status != wantStatus {
		return fmt.Errorf("witness report status differs from retained evidence")
	}
	return nil
}

func validateRetainedBindings(context FrozenWitnessContext, sources, tools []BindingEvidence) error {
	refs := append(append(append([]semanticir.ArtifactRef(nil), context.ReferenceArtifacts...), context.TestArtifacts...), context.EnvironmentArtifacts...)
	if len(sources) != len(refs) || len(tools) != len(context.Tools) {
		return fmt.Errorf("retained artifact/tool binding cardinality differs")
	}
	for index, ref := range refs {
		binding := sources[index]
		if !binding.Verified || binding.ID != ref.ID || binding.ExpectedSHA256 != ref.Digest || binding.ObservedSHA256 != ref.Digest {
			return fmt.Errorf("retained source binding %d is inconsistent", index)
		}
	}
	for index, tool := range context.Tools {
		binding := tools[index]
		if !binding.Verified || binding.ID != tool.Name || binding.ExpectedSHA256 != tool.Digest || binding.ObservedSHA256 != tool.Digest || binding.Version != tool.Version {
			return fmt.Errorf("retained tool binding %d is inconsistent", index)
		}
	}
	return nil
}
