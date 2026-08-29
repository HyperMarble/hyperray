package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// BaselineRetranslationEvidence is the complete, freshly produced frontend
// model and the independently replayed compiler/category proof digests for one
// frozen code artifact. Its shape deliberately mirrors Test IR evidence while
// avoiding an executor -> testir package cycle.
type BaselineRetranslationEvidence struct {
	ArtifactID              string                       `json:"artifact_id"`
	CandidateSHA256         string                       `json:"candidate_sha256"`
	ModelSHA256             string                       `json:"model_sha256"`
	OriginalModelCoreSHA256 string                       `json:"original_model_core_sha256"`
	FreshModelCoreSHA256    string                       `json:"fresh_model_core_sha256"`
	Model                   semanticir.ArtifactModel     `json:"model"`
	Coverage                semanticir.TranslationStatus `json:"coverage"`
	ModelProofSHA256        []string                     `json:"model_proof_sha256"`
}

// BaselineSemanticVector identifies the exact clean-solution vector in the
// authoritative static test predicate. CandidateSHA256 is Digest(Choices);
// the suite/predicate digests bind the proof-side semantic classification.
type BaselineSemanticVector struct {
	ID                    string                      `json:"id"`
	Choices               []semanticir.BehaviorChoice `json:"choices"`
	CandidateSHA256       string                      `json:"candidate_sha256"`
	Baseline              bool                        `json:"baseline"`
	TestsPass             bool                        `json:"tests_pass"`
	TestSuiteSHA256       string                      `json:"test_suite_sha256"`
	StaticPredicateSHA256 string                      `json:"static_predicate_sha256"`
}

// BaselineWitnessPlan is produced by orchestration after a fresh frontend
// retranslation proves that a proof witness is exactly the frozen clean
// solution vector. It contains no edit and no harness.
type BaselineWitnessPlan struct {
	ID              string                          `json:"id"`
	WitnessID       string                          `json:"witness_id"`
	Obligation      semanticir.ProofObligation      `json:"obligation"`
	Witness         semanticir.Counterexample       `json:"witness"`
	Workspace       ProbeWorkspace                  `json:"workspace"`
	SourceArtifacts []semanticir.ArtifactRef        `json:"source_artifacts"`
	Translators     []semanticir.ToolRef            `json:"translators"`
	Verifier        semanticir.ToolRef              `json:"verifier"`
	Retranslations  []BaselineRetranslationEvidence `json:"retranslations"`
	Vector          BaselineSemanticVector          `json:"vector"`
	Execution       TaskEnvironment                 `json:"execution"`
}

// BaselineWitnessEvidence is the certificate-facing typed confirmation. The
// full plan is retained so evidence cannot be detached from a frontend model,
// source/tool/workspace binding, or semantic vector.
type BaselineWitnessEvidence struct {
	Plan                     BaselineWitnessPlan `json:"plan"`
	PlanSHA256               string              `json:"plan_sha256"`
	WitnessSHA256            string              `json:"witness_sha256"`
	VectorSHA256             string              `json:"vector_sha256"`
	ExecutionSHA256          string              `json:"execution_sha256"`
	WorkspaceSHA256          string              `json:"workspace_sha256"`
	SourceBindings           []BindingEvidence   `json:"source_bindings"`
	TranslatorBindings       []BindingEvidence   `json:"translator_bindings"`
	VerifierBinding          BindingEvidence     `json:"verifier_binding"`
	BindingsVerifiedAfterRun bool                `json:"bindings_verified_after_run"`
	SemanticVectorMatch      bool                `json:"semantic_vector_match"`
	Error                    string              `json:"error,omitempty"`
}

type preparedBaselineWitness struct {
	plan        BaselineWitnessPlan
	root        string
	execution   TaskEnvironment
	sources     []preparedBinding
	translators []preparedBinding
	verifier    preparedBinding
}

// ConfirmBaselineWitnesses confirms typed proof witnesses that exactly equal
// the frozen solution vector. Every witness gets a separate fresh workspace
// copy and clean verifier run; no no-op source edit or direct probe is used.
func ConfirmBaselineWitnesses(ctx context.Context, task TaskEnvironment, plans []BaselineWitnessPlan) (report Report) {
	report.Status = StatusProofBlocked
	report.Vacuous = len(plans) == 0
	if len(plans) == 0 {
		return ConfirmIsolated(ctx, task, nil)
	}
	if ctx == nil {
		report.Blockers = append(report.Blockers, Blocker{Stage: "baseline-witness", Code: "nil-context", Detail: "execution context is nil"})
		return report
	}
	prepared, blockers := prepareBaselineWitnessPlans(task, plans)
	if len(blockers) != 0 {
		report.Blockers = append(report.Blockers, blockers...)
		return report
	}
	defer func() { report.Status = aggregateStatus(report) }()
	for index, candidate := range prepared {
		confirmation := confirmBaselineWitness(ctx, candidate)
		if index == 0 {
			report.Baseline = confirmation.Command
			report.BaselineIsolation = confirmation.Isolation
		}
		report.Confirmations = append(report.Confirmations, confirmation)
		report.Blockers = append(report.Blockers, confirmation.Blockers...)
		if confirmation.Isolation == nil || !confirmation.Isolation.IsolatedRemoved || !confirmation.Isolation.OriginalIntact || confirmation.BaselineWitness == nil || !confirmation.BaselineWitness.BindingsVerifiedAfterRun {
			break
		}
	}
	return report
}

func prepareBaselineWitnessPlans(task TaskEnvironment, plans []BaselineWitnessPlan) ([]preparedBaselineWitness, []Blocker) {
	resolvedWork, resolvedRoot, blocker := validateTask(task)
	if blocker != nil {
		return nil, []Blocker{*blocker}
	}
	task.WorkDir, task.WorkspaceRoot = resolvedWork, resolvedRoot
	if !validDigest(task.WorkspaceSHA256) {
		return nil, []Blocker{{Stage: "configuration", Code: "missing-workspace-digest", Detail: "baseline-witness confirmation requires a frozen workspace digest"}}
	}
	current, err := WorkspaceDigest(resolvedRoot)
	if err != nil || current != task.WorkspaceSHA256 {
		detail := fmt.Sprintf("workspace digest is %s, want %s", current, task.WorkspaceSHA256)
		if err != nil {
			detail = err.Error()
		}
		return nil, []Blocker{{Stage: "configuration", Code: "stale-workspace", Detail: detail}}
	}
	ids, witnesses := map[string]bool{}, map[string]bool{}
	result := make([]preparedBaselineWitness, 0, len(plans))
	for _, plan := range plans {
		prepared, planBlocker := prepareBaselineWitnessPlan(task, plan)
		if planBlocker != nil {
			return nil, []Blocker{*planBlocker}
		}
		if ids[plan.ID] || witnesses[plan.WitnessID] {
			return nil, []Blocker{{Stage: "baseline-witness", PlanID: plan.ID, WitnessID: plan.WitnessID, Code: "duplicate-baseline-witness", Detail: "plan and witness IDs must be unique"}}
		}
		ids[plan.ID], witnesses[plan.WitnessID] = true, true
		result = append(result, prepared)
	}
	return result, nil
}

func prepareBaselineWitnessPlan(task TaskEnvironment, plan BaselineWitnessPlan) (preparedBaselineWitness, *Blocker) {
	block := func(code, detail string) (preparedBaselineWitness, *Blocker) {
		return preparedBaselineWitness{}, &Blocker{Stage: "baseline-witness", PlanID: plan.ID, WitnessID: plan.WitnessID, Code: code, Detail: detail}
	}
	if err := validateBaselineWitnessPlanRecord(plan); err != nil {
		return block("invalid-baseline-witness", err.Error())
	}
	root, err := filepath.EvalSymlinks(plan.Workspace.Root)
	if err != nil || filepath.Clean(root) != task.WorkspaceRoot || plan.Workspace.TreeSHA256 != task.WorkspaceSHA256 {
		return block("baseline-witness-workspace-mismatch", "plan workspace does not equal the task's frozen solution workspace")
	}
	planWork, planRoot, planTaskBlocker := validateTask(plan.Execution)
	if planTaskBlocker != nil {
		return block("invalid-baseline-witness-command", planTaskBlocker.Detail)
	}
	plan.Execution.WorkDir, plan.Execution.WorkspaceRoot = planWork, planRoot
	if planRoot != task.WorkspaceRoot || plan.Execution.WorkspaceSHA256 != task.WorkspaceSHA256 || !sameTaskExecution(plan.Execution, task) {
		return block("baseline-witness-command-mismatch", "plan execution differs from the exact task-declared verifier")
	}

	sources := make([]preparedBinding, 0, len(plan.SourceArtifacts))
	resolvedSources := make(map[string]bool, len(plan.SourceArtifacts))
	for _, source := range plan.SourceArtifacts {
		path, err := resolveProbeExisting(task.WorkspaceRoot, source.Path, false)
		if err != nil {
			return block("baseline-source-path", err.Error())
		}
		body, err := os.ReadFile(path)
		if err != nil || digestBytes(body) != source.Digest {
			return block("stale-baseline-source", fmt.Sprintf("source %q differs from its frozen digest", source.ID))
		}
		if resolvedSources[path] {
			return block("duplicate-baseline-source", "source artifact paths resolve to the same file")
		}
		resolvedSources[path] = true
		sources = append(sources, preparedBinding{ref: source, path: path, digest: source.Digest})
	}
	translators := make([]preparedBinding, 0, len(plan.Translators))
	resolvedTranslators := make(map[string]bool, len(plan.Translators))
	for _, tool := range plan.Translators {
		path, err := filepath.EvalSymlinks(tool.Path)
		if err != nil {
			return block("baseline-translator-path", err.Error())
		}
		info, err := os.Lstat(path)
		body, readErr := os.ReadFile(path)
		if err != nil || readErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || digestBytes(body) != tool.Digest {
			return block("stale-baseline-translator", fmt.Sprintf("translator %q differs from its frozen executable", tool.Name))
		}
		if resolvedTranslators[path] {
			return block("duplicate-baseline-translator", "translator paths resolve to the same executable")
		}
		resolvedTranslators[path] = true
		translators = append(translators, preparedBinding{tool: tool, path: filepath.Clean(path), digest: tool.Digest, version: tool.Version})
	}
	verifierPath, err := filepath.EvalSymlinks(plan.Verifier.Path)
	if err != nil {
		return block("baseline-verifier-path", err.Error())
	}
	verifierInfo, statErr := os.Lstat(verifierPath)
	verifierBody, readErr := os.ReadFile(verifierPath)
	if statErr != nil || readErr != nil || !verifierInfo.Mode().IsRegular() || verifierInfo.Mode().Perm()&0o111 == 0 || digestBytes(verifierBody) != plan.Verifier.Digest {
		return block("stale-baseline-verifier", "verifier differs from its frozen executable")
	}
	verifier := preparedBinding{tool: plan.Verifier, path: filepath.Clean(verifierPath), digest: plan.Verifier.Digest, version: plan.Verifier.Version}
	return preparedBaselineWitness{plan: plan, root: task.WorkspaceRoot, execution: task, sources: sources, translators: translators, verifier: verifier}, nil
}

func confirmBaselineWitness(ctx context.Context, prepared preparedBaselineWitness) Confirmation {
	plan := prepared.plan
	confirmation := Confirmation{
		WitnessID: plan.WitnessID, Mode: ConfirmationModeBaselineWitness, Status: StatusProofBlocked,
		ExpectedTestPasses: true,
	}
	observed := false
	confirmation.ObservedTestPasses = &observed
	evidence := &BaselineWitnessEvidence{
		Plan: plan, PlanSHA256: mustProbeDigest(plan), WitnessSHA256: mustProbeDigest(plan.Witness),
		VectorSHA256: mustProbeDigest(plan.Vector), ExecutionSHA256: mustProbeDigest(plan.Execution),
		WorkspaceSHA256: plan.Workspace.TreeSHA256, SemanticVectorMatch: true,
	}
	confirmation.BaselineWitness = evidence
	for _, source := range prepared.sources {
		evidence.SourceBindings = append(evidence.SourceBindings, BindingEvidence{ID: source.ref.ID, Path: source.path, ExpectedSHA256: source.digest, ObservedSHA256: source.digest, Verified: true})
	}
	for _, tool := range prepared.translators {
		evidence.TranslatorBindings = append(evidence.TranslatorBindings, BindingEvidence{ID: tool.tool.Name, Path: tool.path, ExpectedSHA256: tool.digest, ObservedSHA256: tool.digest, Version: tool.version, Verified: true})
	}
	evidence.VerifierBinding = BindingEvidence{ID: prepared.verifier.tool.Name, Path: prepared.verifier.path, ExpectedSHA256: prepared.verifier.digest, ObservedSHA256: prepared.verifier.digest, Version: prepared.verifier.version, Verified: true}
	command, isolation, blocker := runIsolatedBaseline(ctx, prepared.execution)
	confirmation.Command, confirmation.Isolation = command, isolation
	if blocker != nil {
		blocker.PlanID, blocker.WitnessID = plan.ID, plan.WitnessID
		confirmation.Blockers = append(confirmation.Blockers, *blocker)
		evidence.Error = blocker.Detail
		return confirmation
	}
	if err := verifyBaselineBindings(prepared); err != nil {
		evidence.Error = err.Error()
		confirmation.Blockers = append(confirmation.Blockers, Blocker{Stage: "baseline-witness", PlanID: plan.ID, WitnessID: plan.WitnessID, Code: "baseline-binding-changed", Detail: err.Error()})
		return confirmation
	}
	evidence.BindingsVerifiedAfterRun = true
	observed = command.Passed
	confirmation.ObservedTestPasses = &observed
	if command.Passed {
		confirmation.Status = StatusConfirmed
	} else {
		detail := "frozen verifier rejected the clean semantic baseline although the witness model requires it to pass"
		evidence.Error = detail
		confirmation.Blockers = append(confirmation.Blockers, Blocker{
			Stage: "baseline-witness", PlanID: plan.ID, WitnessID: plan.WitnessID,
			Code: "model-execution-mismatch", Detail: detail,
		})
		confirmation.Status = StatusProofBlocked
	}
	return confirmation
}

func verifyBaselineBindings(prepared preparedBaselineWitness) error {
	for _, source := range prepared.sources {
		body, err := os.ReadFile(source.path)
		if err != nil || digestBytes(body) != source.digest {
			return fmt.Errorf("source %q changed during baseline witness confirmation", source.ref.ID)
		}
	}
	if err := verifyPreparedProbeTools(prepared.translators); err != nil {
		return err
	}
	return verifyPreparedProbeTools([]preparedBinding{prepared.verifier})
}

func validateBaselineWitnessPlanRecord(plan BaselineWitnessPlan) error {
	if plan.ID == "" || plan.WitnessID == "" || plan.Witness.ID != plan.WitnessID || plan.Witness.Obligation != plan.Obligation {
		return fmt.Errorf("baseline witness plan has incomplete identity")
	}
	if plan.Obligation != semanticir.ObligationReferenceCorrectness && plan.Obligation != semanticir.ObligationTestsSound {
		return fmt.Errorf("baseline witness has unsupported obligation %q", plan.Obligation)
	}
	if !plan.Witness.TestPasses || plan.Workspace.ID == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || !validDigest(plan.Workspace.TreeSHA256) || !filepath.IsAbs(plan.Workspace.Root) || filepath.Clean(plan.Workspace.Root) != plan.Workspace.Root {
		return fmt.Errorf("baseline witness does not bind a passing solution+new-tests workspace")
	}
	if len(plan.SourceArtifacts) == 0 || len(plan.Translators) == 0 || len(plan.Retranslations) != len(plan.SourceArtifacts) {
		return fmt.Errorf("baseline witness omits source, translator, or retranslation evidence")
	}
	if plan.Verifier.Name == "" || plan.Verifier.Version == "" || !filepath.IsAbs(plan.Verifier.Path) || !validDigest(plan.Verifier.Digest) || len(plan.Execution.Command) == 0 || plan.Execution.Command[0] != plan.Verifier.Path {
		return fmt.Errorf("baseline witness has no exact frozen verifier binding")
	}
	if plan.Vector.ID == "" || !plan.Vector.Baseline || !plan.Vector.TestsPass || !validDigest(plan.Vector.TestSuiteSHA256) || !validDigest(plan.Vector.StaticPredicateSHA256) {
		return fmt.Errorf("baseline witness does not bind the passing static test predicate")
	}
	choiceDigest, err := semanticir.Digest(plan.Vector.Choices)
	if err != nil || choiceDigest != plan.Vector.CandidateSHA256 || !sameSemanticChoices(plan.Vector.Choices, plan.Witness.Choices) {
		return fmt.Errorf("baseline semantic vector digest or witness choices differ")
	}
	if !validDigest(plan.Witness.Provenance.ArtifactDigest) || plan.Witness.Provenance.ArtifactID == "" || (plan.Witness.Provenance.Translation != semanticir.TranslationTranslated && plan.Witness.Provenance.Translation != semanticir.TranslationComplete) {
		return fmt.Errorf("baseline proof witness has incomplete independent provenance")
	}
	if len(plan.Witness.ObservedOutcomes) != len(plan.Vector.Choices) {
		return fmt.Errorf("baseline witness observed outcome vector is incomplete")
	}
	for index, choice := range plan.Vector.Choices {
		if choice.Behavior.OperationID == "" || choice.OutcomeID == "" || plan.Witness.ObservedOutcomes[index] != choice.OutcomeID {
			return fmt.Errorf("baseline vector choice %d is incomplete", index)
		}
		if index > 0 && baselineChoiceKey(plan.Vector.Choices[index-1]) >= baselineChoiceKey(choice) {
			return fmt.Errorf("baseline vector choices are not in strict canonical behavior order")
		}
	}

	sourceByID, sourcePaths, translatorByName, translatorPaths := map[string]semanticir.ArtifactRef{}, map[string]bool{}, map[string]semanticir.ToolRef{}, map[string]bool{}
	for _, source := range plan.SourceArtifacts {
		if source.ID == "" || source.Kind != semanticir.ArtifactCode || source.Path == "" || !validDigest(source.Digest) || sourceByID[source.ID].ID != "" || sourcePaths[source.Path] {
			return fmt.Errorf("baseline witness has an invalid or duplicate code source")
		}
		sourceByID[source.ID] = source
		sourcePaths[source.Path] = true
	}
	for _, tool := range plan.Translators {
		if tool.Name == "" || tool.Path == "" || tool.Version == "" || !filepath.IsAbs(tool.Path) || !validDigest(tool.Digest) || translatorByName[tool.Name].Name != "" || translatorPaths[tool.Path] {
			return fmt.Errorf("baseline witness has an invalid or duplicate translator")
		}
		translatorByName[tool.Name] = tool
		translatorPaths[tool.Path] = true
	}
	modeledChoices := make([]semanticir.BehaviorChoice, 0, len(plan.Vector.Choices))
	seenArtifacts := map[string]bool{}
	for _, fresh := range plan.Retranslations {
		model := fresh.Model
		source := sourceByID[fresh.ArtifactID]
		if source.ID == "" || seenArtifacts[fresh.ArtifactID] || model.Artifact != source || model.Kind != semanticir.ArtifactCode || fresh.CandidateSHA256 != source.Digest || fresh.Coverage != semanticir.TranslationComplete || model.Coverage.Status != semanticir.TranslationComplete || len(model.Coverage.Unsupported) != 0 {
			return fmt.Errorf("baseline retranslation %q is incomplete or source-detached", fresh.ArtifactID)
		}
		seenArtifacts[fresh.ArtifactID] = true
		freshCore, coreErr := semanticir.ArtifactModelTranslationDigest(model)
		if !reflect.DeepEqual(translatorByName[model.Translator.Name], model.Translator) || fresh.ModelSHA256 != mustProbeDigest(model) || coreErr != nil || !validDigest(fresh.OriginalModelCoreSHA256) || fresh.FreshModelCoreSHA256 != freshCore || fresh.OriginalModelCoreSHA256 != fresh.FreshModelCoreSHA256 || semanticir.HasErrors(semanticir.ValidateArtifactModel(model)) {
			return fmt.Errorf("baseline retranslation %q is invalid or translator-detached", fresh.ArtifactID)
		}
		if !reflect.DeepEqual(fresh.ModelProofSHA256, baselineModelProofDigests(model)) || len(fresh.ModelProofSHA256) == 0 {
			return fmt.Errorf("baseline retranslation %q proof digests differ from its full model", fresh.ArtifactID)
		}
		for _, proof := range model.CompilerEvidence {
			if proof.SourceDigest != source.Digest || proof.WorkspaceTreeDigest != plan.Workspace.TreeSHA256 || !reflect.DeepEqual(proof.Tool, model.Translator) {
				return fmt.Errorf("baseline retranslation %q compiler proof is detached from source/workspace/tool", fresh.ArtifactID)
			}
		}
		for _, proof := range model.ExhaustiveEvidence {
			if proof.SourceDigest != source.Digest || proof.WorkspaceTreeDigest != plan.Workspace.TreeSHA256 || !reflect.DeepEqual(proof.Tool, model.Translator) || !proof.Complete {
				return fmt.Errorf("baseline retranslation %q execution proof is detached from source/workspace/tool", fresh.ArtifactID)
			}
		}
		for _, behaviorCase := range model.Cases {
			if len(behaviorCase.OutcomeIDs) != 1 {
				return fmt.Errorf("baseline model case %q does not select exactly one clean outcome", behaviorCase.ID)
			}
			modeledChoices = append(modeledChoices, semanticir.BehaviorChoice{Behavior: semanticir.BehaviorRef{OperationID: behaviorCase.OperationID, Conditions: behaviorCase.Conditions, Provenance: behaviorCase.Provenance}, OutcomeID: behaviorCase.OutcomeIDs[0]})
		}
	}
	if len(seenArtifacts) != len(sourceByID) {
		return fmt.Errorf("baseline retranslations do not cover every frozen code source")
	}
	sort.Slice(modeledChoices, func(i, j int) bool {
		return baselineChoiceKey(modeledChoices[i]) < baselineChoiceKey(modeledChoices[j])
	})
	if !sameSemanticChoices(modeledChoices, plan.Vector.Choices) {
		return fmt.Errorf("baseline vector is not the exact full freshly retranslated model vector")
	}
	return nil
}

func baselineChoiceKey(choice semanticir.BehaviorChoice) string {
	value, _ := semanticir.CanonicalJSON(struct {
		OperationID string                        `json:"operation_id"`
		Conditions  semanticir.Assignment         `json:"conditions"`
		Inputs      map[string]semanticir.Literal `json:"inputs"`
	}{choice.Behavior.OperationID, choice.Behavior.Conditions, choice.Behavior.Inputs})
	return string(value)
}

func sameSemanticChoices(left, right []semanticir.BehaviorChoice) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Behavior.OperationID != right[index].Behavior.OperationID || !reflect.DeepEqual(left[index].Behavior.Conditions, right[index].Behavior.Conditions) || !reflect.DeepEqual(left[index].Behavior.Inputs, right[index].Behavior.Inputs) || left[index].OutcomeID != right[index].OutcomeID {
			return false
		}
	}
	return true
}

func baselineModelProofDigests(model semanticir.ArtifactModel) []string {
	result := make([]string, 0, len(model.CompilerEvidence)+len(model.ExhaustiveEvidence))
	for _, evidence := range model.CompilerEvidence {
		result = append(result, mustProbeDigest(evidence))
	}
	for _, evidence := range model.ExhaustiveEvidence {
		result = append(result, mustProbeDigest(evidence))
	}
	return result
}

func sameTaskExecution(left, right TaskEnvironment) bool {
	return reflect.DeepEqual(left.Command, right.Command) && left.WorkDir == right.WorkDir && left.WorkspaceRoot == right.WorkspaceRoot &&
		left.WorkspaceSHA256 == right.WorkspaceSHA256 && left.Timeout == right.Timeout && reflect.DeepEqual(left.Environment, right.Environment) &&
		left.ExactEnvironment == right.ExactEnvironment && reflect.DeepEqual(left.PassSignal, right.PassSignal)
}

// ValidateBaselineWitnessConfirmation checks a confirmed baseline-equal
// record without accepting a no-op edit or probe as a substitute.
func ValidateBaselineWitnessConfirmation(confirmation Confirmation) error {
	if confirmation.Mode != ConfirmationModeBaselineWitness || confirmation.Status != StatusConfirmed || confirmation.WitnessID == "" || len(confirmation.PlanIDs) != 0 || len(confirmation.Plans) != 0 || len(confirmation.Materializations) != 0 || confirmation.Probe != nil || confirmation.BaselineWitness == nil || len(confirmation.Blockers) != 0 {
		return fmt.Errorf("baseline-witness confirmation mixes evidence modes or is incomplete")
	}
	evidence := confirmation.BaselineWitness
	plan := evidence.Plan
	if plan.WitnessID != confirmation.WitnessID {
		return fmt.Errorf("baseline-witness confirmation has an invalid retained plan")
	}
	if err := validateBaselineWitnessPlanRecord(plan); err != nil {
		return fmt.Errorf("baseline-witness confirmation has an invalid retained plan: %w", err)
	}
	if evidence.PlanSHA256 != mustProbeDigest(plan) || evidence.WitnessSHA256 != mustProbeDigest(plan.Witness) || evidence.VectorSHA256 != mustProbeDigest(plan.Vector) || evidence.ExecutionSHA256 != mustProbeDigest(plan.Execution) || evidence.WorkspaceSHA256 != plan.Workspace.TreeSHA256 || !evidence.SemanticVectorMatch || evidence.Error != "" {
		return fmt.Errorf("baseline-witness digests or semantic match are inconsistent")
	}
	if confirmation.ObservedTestPasses == nil || !confirmation.ExpectedTestPasses || !*confirmation.ObservedTestPasses || !confirmation.Command.Passed {
		return fmt.Errorf("baseline-witness clean verifier did not pass")
	}
	if err := validateProbeCommandEvidence(confirmation.Command); err != nil {
		return err
	}
	if confirmation.Isolation == nil {
		return fmt.Errorf("baseline-witness isolation record is missing")
	}
	if err := validateIsolationRecord(*confirmation.Isolation, plan); err != nil {
		return fmt.Errorf("baseline-witness isolation: %v", err)
	}
	if len(evidence.SourceBindings) != len(plan.SourceArtifacts) || len(evidence.TranslatorBindings) != len(plan.Translators) || !evidence.BindingsVerifiedAfterRun {
		return fmt.Errorf("baseline-witness binding evidence cardinality differs")
	}
	for index, binding := range evidence.SourceBindings {
		ref := plan.SourceArtifacts[index]
		if !binding.Verified || binding.ID != ref.ID || binding.ExpectedSHA256 != ref.Digest || binding.ObservedSHA256 != ref.Digest {
			return fmt.Errorf("baseline source binding %d is inconsistent", index)
		}
	}
	for index, binding := range evidence.TranslatorBindings {
		tool := plan.Translators[index]
		if !binding.Verified || binding.ID != tool.Name || binding.ExpectedSHA256 != tool.Digest || binding.ObservedSHA256 != tool.Digest || binding.Version != tool.Version {
			return fmt.Errorf("baseline translator binding %d is inconsistent", index)
		}
	}
	if !evidence.VerifierBinding.Verified || evidence.VerifierBinding.ID != plan.Verifier.Name || evidence.VerifierBinding.ExpectedSHA256 != plan.Verifier.Digest || evidence.VerifierBinding.ObservedSHA256 != plan.Verifier.Digest || evidence.VerifierBinding.Version != plan.Verifier.Version {
		return fmt.Errorf("baseline verifier binding is inconsistent")
	}
	if err := validateBaselineWitnessCommand(confirmation.Command, *confirmation.Isolation, plan.Execution); err != nil {
		return err
	}
	return nil
}

func validateBaselineWitnessCommand(command CommandEvidence, isolation IsolationEvidence, execution TaskEnvironment) error {
	root, err := filepath.EvalSymlinks(execution.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve baseline command workspace: %w", err)
	}
	if filepath.Clean(root) != isolation.OriginalRoot {
		return fmt.Errorf("baseline isolation original root differs from command workspace")
	}
	work, err := filepath.EvalSymlinks(execution.WorkDir)
	if err != nil || !pathWithin(root, work) {
		return fmt.Errorf("baseline command workdir is outside its workspace")
	}
	relativeWork, _ := filepath.Rel(root, work)
	want := execution
	want.WorkspaceRoot = isolation.IsolatedRoot
	want.WorkDir = filepath.Join(isolation.IsolatedRoot, relativeWork)
	if want.PassSignal.VerdictFile != nil {
		declared := want.PassSignal.VerdictFile.Path
		if filepath.IsAbs(declared) {
			relative, relErr := filepath.Rel(root, declared)
			if relErr != nil || !pathWithin(root, declared) {
				return fmt.Errorf("baseline command verdict is outside its workspace")
			}
			declared = filepath.Join(isolation.IsolatedRoot, relative)
		}
		want.PassSignal = VerdictFileSignal(declared, want.PassSignal.VerdictFile.PassValue)
	}
	wantDigest, _ := semanticir.Digest(want)
	if !reflect.DeepEqual(command.Command, execution.Command) || command.WorkDir != want.WorkDir || command.Timeout != execution.Timeout || command.EnvironmentSHA256 != digestEnvironment(execution.Environment) || command.CommandSHA256 != wantDigest {
		return fmt.Errorf("baseline command differs from exact argv/workdir/environment/timeout binding")
	}
	if execution.PassSignal.ExitCode != nil {
		if command.Signal.Kind != "exit-code" || command.Signal.ExpectedExitCode == nil || *command.Signal.ExpectedExitCode != *execution.PassSignal.ExitCode {
			return fmt.Errorf("baseline command exit signal differs from plan")
		}
	} else {
		verdict := execution.PassSignal.VerdictFile
		if verdict == nil || command.Signal.Kind != "verdict-file" || command.Signal.ExpectedValueSHA256 != digestBytes([]byte(strings.TrimSpace(verdict.PassValue))) {
			return fmt.Errorf("baseline command verdict signal differs from plan")
		}
	}
	return nil
}

func validateIsolationRecord(isolation IsolationEvidence, plan BaselineWitnessPlan) error {
	if isolation.OriginalRoot == "" || isolation.IsolatedRoot == "" || isolation.OriginalRoot == isolation.IsolatedRoot || isolation.ExpectedSHA256 != plan.Workspace.TreeSHA256 || isolation.OriginalBeforeSHA256 != plan.Workspace.TreeSHA256 || isolation.CopyBeforeSHA256 != plan.Workspace.TreeSHA256 || isolation.OriginalAfterSHA256 != plan.Workspace.TreeSHA256 || !validDigest(isolation.CopyAfterSHA256) || !isolation.IsolatedRemoved || !isolation.OriginalIntact || isolation.Error != "" {
		return fmt.Errorf("fresh workspace copy/removal digests are incomplete")
	}
	return nil
}
