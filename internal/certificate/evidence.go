package certificate

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	proofengine "github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

const (
	enumerationProofMethod = "exhaustive-finite-enumeration"
	smtProofMethod         = "z3-qf-lia"
)

// validateSemanticEvidence checks the complete semantic task against the
// frozen manifest. This is deliberately stronger than checking IRDigest: it
// proves that every source, tool, and workspace identity retained in the typed
// proof input is the one actively frozen by taskbundle.
func validateSemanticEvidence(document Document) error {
	task := &document.SemanticIR
	digest, err := semanticir.Digest(task)
	if err != nil {
		return fmt.Errorf("certificate semantic IR: %w", err)
	}
	if digest != document.IRDigest {
		return errors.New("certificate semantic IR digest mismatch")
	}
	independent := []struct {
		name       string
		got        string
		canonical  func(*semanticir.Task) (string, error)
		validation func(*semanticir.Task) []semanticir.Diagnostic
	}{
		{"Spec", document.SpecIRDigest, semanticir.CanonicalSpecIRDigest, semanticir.ValidateSpecIRDigest},
		{"reference", document.ReferenceIRDigest, semanticir.CanonicalReferenceIRDigest, semanticir.ValidateReferenceIR},
		{"Test", document.TestIRDigest, semanticir.CanonicalTestIRDigest, semanticir.ValidateTestIR},
		{"environment", document.EnvironmentIRDigest, semanticir.CanonicalEnvironmentIRDigest, semanticir.ValidateEnvironmentIR},
	}
	for _, model := range independent {
		want, digestErr := model.canonical(task)
		if digestErr != nil || !validDigest(model.got) || model.got != want {
			return fmt.Errorf("certificate canonical independent %s IR digest mismatch", model.name)
		}
		if diagnostics := model.validation(task); semanticir.HasErrors(diagnostics) {
			return fmt.Errorf("certificate independent %s IR is stale, incomplete, or invalid: %s", model.name, diagnostics[0].Message)
		}
	}
	if task.SpecIRDigest != document.SpecIRDigest {
		return errors.New("certificate embedded Spec IR digest differs from its independent binding")
	}
	if diagnostics := task.Validate(); semanticir.HasErrors(diagnostics) {
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == semanticir.SeverityError {
				return fmt.Errorf("certificate semantic IR is invalid: %s: %s", diagnostic.Code, diagnostic.Message)
			}
		}
		return errors.New("certificate semantic IR is invalid")
	}

	frozenEnvironmentSources := make(map[string]taskbundle.Artifact)
	for _, artifact := range document.Manifest.Artifacts {
		if artifact.Kind == string(semanticir.ArtifactEnvironment) {
			frozenEnvironmentSources[artifact.ID] = artifact
		}
	}
	seen := map[string]semanticir.ArtifactRef{}
	refs := []semanticir.ArtifactRef{task.Instruction, task.Spec}
	if task.Environment != nil {
		if task.Environment.Configuration.Kind != semanticir.ArtifactConfiguration {
			return errors.New("certificate semantic environment has no separately frozen configuration artifact")
		}
		if task.Environment.Artifact != (semanticir.ArtifactRef{}) && task.Environment.Artifact != task.Environment.Configuration {
			return errors.New("certificate semantic environment legacy artifact alias differs from configuration")
		}
		if len(task.Environment.SourceArtifacts) != len(frozenEnvironmentSources) {
			return fmt.Errorf("certificate semantic environment binds %d environment sources, frozen manifest has %d", len(task.Environment.SourceArtifacts), len(frozenEnvironmentSources))
		}
		seenSources := map[string]bool{}
		for _, source := range task.Environment.SourceArtifacts {
			artifact, ok := frozenEnvironmentSources[source.ID]
			if !ok || source.Kind != semanticir.ArtifactEnvironment || artifact.Path != filepath.ToSlash(filepath.Clean(source.Path)) || artifact.SHA256 != source.Digest || seenSources[source.ID] {
				return fmt.Errorf("certificate semantic environment source %q is missing, duplicate, relabeled, or stale", source.ID)
			}
			seenSources[source.ID] = true
		}
		refs = append(refs, task.Environment.Configuration)
		refs = append(refs, task.Environment.SourceArtifacts...)
	}
	for _, model := range task.Artifacts {
		refs = append(refs, model.Artifact)
		if model.RunnerSelection != nil {
			refs = append(refs, model.RunnerSelection.Configuration)
		}
	}
	for _, ref := range refs {
		if previous, exists := seen[ref.ID]; exists {
			if previous != ref {
				return fmt.Errorf("certificate semantic IR gives frozen artifact %q conflicting identities", ref.ID)
			}
			continue
		}
		seen[ref.ID] = ref
		if !artifactRefFrozen(ref, document.Manifest) {
			return fmt.Errorf("certificate semantic artifact %q is not bound to the frozen manifest", ref.ID)
		}
	}

	if task.Environment == nil {
		return errors.New("certificate semantic IR has no environment")
	}
	if task.Environment.Identity != document.Manifest.Environment.Identity {
		return errors.New("certificate semantic environment identity differs from frozen manifest")
	}
	configurationDigest, err := semanticir.Digest(document.Manifest.Environment.Configuration)
	if err != nil {
		return fmt.Errorf("certificate environment configuration: %w", err)
	}
	if task.Environment.ConfigDigest != configurationDigest {
		return errors.New("certificate semantic environment configuration digest mismatch")
	}
	if err := validateSemanticTools(task.Environment.Tools, document.Manifest.Environment.Tools); err != nil {
		return err
	}
	if err := validateSemanticWorkspaces(task.Environment.Commands, document.Manifest.Workspaces, task.Environment.Tools); err != nil {
		return err
	}
	for _, model := range task.Artifacts {
		if err := requireFrozenTool(model.Translator, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("artifact %q translator: %w", model.Artifact.ID, err)
		}
		for _, evidence := range model.CompilerEvidence {
			if evidence.SourceDigest != model.Artifact.Digest {
				return fmt.Errorf("compiler evidence %q is stale for artifact %q", evidence.ID, model.Artifact.ID)
			}
			if len(evidence.Argv) == 0 || evidence.Argv[0] != evidence.Tool.Path {
				return fmt.Errorf("compiler evidence %q invocation is not bound to its exact tool path", evidence.ID)
			}
			if err := requireFrozenTool(evidence.Tool, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("compiler evidence %q tool: %w", evidence.ID, err)
			}
			if err := requireFrozenTool(evidence.Prover, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("compiler evidence %q prover: %w", evidence.ID, err)
			}
			if !workspaceTreeFrozen(evidence.WorkspaceTreeDigest, task.Environment.Commands) {
				return fmt.Errorf("compiler evidence %q has a stale workspace binding", evidence.ID)
			}
			for _, partition := range evidence.Partitions {
				proofs := []semanticir.ReplayableProof{partition.TotalityProof, partition.DisjointnessProof}
				for _, label := range partition.Labels {
					proofs = append(proofs, label.ReachabilityProof)
				}
				for _, exclusion := range partition.Exclusions {
					proofs = append(proofs, exclusion.Proof)
				}
				for _, replay := range proofs {
					if replay.Prover != evidence.Prover || replay.EnvironmentDigest != evidence.EnvironmentDigest {
						return fmt.Errorf("compiler evidence %q has a replay proof with stale prover/environment bindings", evidence.ID)
					}
					if err := requireFrozenTool(replay.Prover, document.Manifest.Environment.Tools); err != nil {
						return fmt.Errorf("compiler evidence %q replay prover: %w", evidence.ID, err)
					}
				}
			}
		}
		for _, evidence := range model.ExhaustiveEvidence {
			if evidence.SourceDigest != model.Artifact.Digest || !workspaceEvidenceFrozen(evidence.WorkspaceTreeDigest, evidence.EnvironmentDigest, task.Environment.Commands) {
				return fmt.Errorf("exhaustive execution %q has stale source/workspace/environment bindings", evidence.ID)
			}
			if err := requireFrozenTool(evidence.Tool, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("exhaustive execution %q tool: %w", evidence.ID, err)
			}
			if err := validateFrozenProbeSteps(evidence.Steps, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("exhaustive execution %q: %w", evidence.ID, err)
			}
			if err := validateFrozenProbeSteps(evidence.Replay.CleanupSteps, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("exhaustive execution %q cleanup: %w", evidence.ID, err)
			}
		}
		if model.TestProjection != nil {
			projection := model.TestProjection
			if projection.Derivation.WorkspaceTreeDigest == "" || !workspaceTreeFrozen(projection.Derivation.WorkspaceTreeDigest, task.Environment.Commands) {
				return fmt.Errorf("test projection %q has a stale workspace binding", model.Artifact.ID)
			}
			if err := requireFrozenTool(projection.Derivation.Tool, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("test projection %q derivation tool: %w", model.Artifact.ID, err)
			}
			if err := validateFrozenProbeSteps(projection.Derivation.Steps, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("test projection %q: %w", model.Artifact.ID, err)
			}
			for _, construct := range projection.Constructs {
				if err := requireFrozenTool(construct.Tool, document.Manifest.Environment.Tools); err != nil {
					return fmt.Errorf("test projection %q construct %q: %w", model.Artifact.ID, construct.ID, err)
				}
			}
			if projection.Extensionality.Prover != (semanticir.ToolRef{}) {
				if err := requireFrozenTool(projection.Extensionality.Prover, document.Manifest.Environment.Tools); err != nil {
					return fmt.Errorf("test projection %q extensionality prover: %w", model.Artifact.ID, err)
				}
			}
		}
		if model.RunnerSelection != nil {
			runner := model.RunnerSelection
			if runner.Configuration.Kind != semanticir.ArtifactConfiguration || !artifactRefFrozen(runner.Configuration, document.Manifest) {
				return fmt.Errorf("test runner %q configuration is not a separately frozen configuration artifact", model.Artifact.ID)
			}
			if err := requireFrozenTool(runner.Verifier, document.Manifest.Environment.Tools); err != nil {
				return fmt.Errorf("test runner %q verifier: %w", model.Artifact.ID, err)
			}
			boundCommand := false
			for _, command := range task.Environment.Commands {
				boundCommand = boundCommand || reflect.DeepEqual(runner.Command, command)
			}
			if !boundCommand {
				return fmt.Errorf("test runner %q command is detached from the frozen workspace triple", model.Artifact.ID)
			}
		}
	}

	if task.TestSuite == nil {
		return errors.New("certificate semantic IR has no authoritative TestSuite")
	}
	if err := requireFrozenTool(task.TestSuite.Verifier, document.Manifest.Environment.Tools); err != nil {
		return fmt.Errorf("test-suite verifier: %w", err)
	}
	if !workspaceEvidenceFrozen(task.TestSuite.Execution.TreeDigest, task.TestSuite.Execution.EnvironmentDigest, task.Environment.Commands) {
		return errors.New("test-suite execution has stale workspace/environment bindings")
	}
	if !exactTestSuiteSources(task.TestSuite.SourceArtifacts, task) {
		return errors.New("test-suite source artifacts truncate or alter frozen configuration/environment/test inputs")
	}
	return nil
}

// validateDerivationReplays retains the full executor records behind every
// compiler semantic graph. The proof transcript deliberately omits volatile
// timestamps and disposable paths; certificates bind that projection back to
// the complete two-run evidence instead of accepting the compact summary as
// an unattested assertion.
func validateDerivationReplays(document Document) error {
	type expectedRecord struct {
		model    semanticir.ArtifactModel
		evidence semanticir.CompilerEvidence
		graph    semanticir.CompilerSemanticGraph
	}
	expected := map[string]expectedRecord{}
	for _, model := range document.SemanticIR.Artifacts {
		for _, evidence := range model.CompilerEvidence {
			if evidence.SemanticGraph == nil {
				continue
			}
			graphDigest, err := semanticir.Digest(*evidence.SemanticGraph)
			if err != nil {
				return fmt.Errorf("certificate compiler graph %q: %w", evidence.ID, err)
			}
			if _, duplicate := expected[graphDigest]; duplicate {
				return fmt.Errorf("certificate semantic IR repeats compiler graph %s", graphDigest)
			}
			expected[graphDigest] = expectedRecord{model: model, evidence: evidence, graph: *evidence.SemanticGraph}
		}
	}
	if len(expected) == 0 {
		if len(document.DerivationReplays) != 0 || document.DerivationReplaysSHA256 != "" {
			return errors.New("certificate carries detached compiler derivation replay evidence")
		}
		return nil
	}
	if len(document.DerivationReplays) != len(expected) || !validDigest(document.DerivationReplaysSHA256) {
		return errors.New("certificate compiler derivation replays are missing or truncated")
	}
	digest, err := semanticir.Digest(document.DerivationReplays)
	if err != nil || digest != document.DerivationReplaysSHA256 {
		return errors.New("certificate compiler derivation replay aggregate digest mismatch")
	}

	solution, ok := frozenSolutionWorkspace(document.Manifest)
	if !ok {
		return errors.New("certificate compiler derivation replays have no frozen solution workspace")
	}
	workspaceID := semanticSolutionWorkspaceID(document, solution.TreeSHA256)
	proofBindings := make(map[string]proofengine.DerivationReplayBinding, len(document.ProofEvidence.Transcript.DerivationReplays))
	for _, binding := range document.ProofEvidence.Transcript.DerivationReplays {
		if binding.GraphSHA256 == "" || proofBindings[binding.GraphSHA256].GraphSHA256 != "" {
			return errors.New("certificate proof transcript has an empty or duplicate compiler derivation replay binding")
		}
		proofBindings[binding.GraphSHA256] = binding
	}
	seen := map[string]bool{}
	for _, replay := range document.DerivationReplays {
		if err := executor.ValidateDerivationReplay(replay); err != nil {
			return fmt.Errorf("certificate compiler derivation replay %q: %w", replay.Plan.ID, err)
		}
		want, exists := expected[replay.GraphSHA256]
		if !exists || seen[replay.GraphSHA256] || !reflect.DeepEqual(replay.Plan.Graph, want.graph) {
			return fmt.Errorf("certificate compiler derivation replay %q is detached, duplicated, or stale", replay.Plan.ID)
		}
		seen[replay.GraphSHA256] = true
		if replay.Plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || replay.Plan.Workspace.TreeSHA256 != solution.TreeSHA256 || replay.Plan.Workspace.ID != workspaceID {
			return fmt.Errorf("certificate compiler derivation replay %q is bound to a stale workspace", replay.Plan.ID)
		}
		if len(replay.Plan.SourceArtifacts) != 1 || replay.Plan.SourceArtifacts[0] != want.model.Artifact || !probeSourceFrozen(replay.Plan.SourceArtifacts[0], document) {
			return fmt.Errorf("certificate compiler derivation replay %q truncates or alters its exact source scope", replay.Plan.ID)
		}
		if replay.Plan.Graph.SourceDigest != want.model.Artifact.Digest || replay.Plan.Graph.Provenance.ArtifactID != want.model.Artifact.ID || replay.Plan.Graph.Provenance.ArtifactDigest != want.model.Artifact.Digest {
			return fmt.Errorf("certificate compiler derivation replay %q graph provenance is detached from its artifact", replay.Plan.ID)
		}
		if err := requireFrozenTool(replay.Plan.Graph.Tool, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("certificate compiler derivation replay %q primary tool: %w", replay.Plan.ID, err)
		}
		if err := validateFrozenProbeSteps(replay.Plan.Graph.DerivationSteps, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("certificate compiler derivation replay %q derivation steps: %w", replay.Plan.ID, err)
		}
		if err := validateFrozenProbeSteps(replay.Plan.Graph.DecoderSteps, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("certificate compiler derivation replay %q decoder steps: %w", replay.Plan.ID, err)
		}
		projection := proofengine.DerivationReplayBinding{
			PlanSHA256: replay.PlanSHA256, GraphSHA256: replay.GraphSHA256, WorkspaceSHA256: replay.WorkspaceSHA256,
			SourceBindings: append([]executor.BindingEvidence(nil), replay.SourceBindings...), ToolBindings: append([]executor.BindingEvidence(nil), replay.ToolBindings...),
			IRSHA256: replay.Plan.Graph.IRDigest, DecoderOutputSHA256: replay.Plan.Graph.DecoderOutputDigest, Repetitions: len(replay.Runs),
			Deterministic: replay.Deterministic, OriginalWorkspaceIntact: replay.OriginalWorkspaceIntact,
		}
		if proofBinding, exists := proofBindings[replay.GraphSHA256]; !exists || !reflect.DeepEqual(projection, proofBinding) {
			return fmt.Errorf("certificate compiler derivation replay %q differs from the proof transcript", replay.Plan.ID)
		}
	}
	if len(seen) != len(expected) || len(proofBindings) != len(expected) {
		return errors.New("certificate compiler derivation replays truncate Semantic IR or proof transcript evidence")
	}
	return nil
}

// validateExhaustiveReplays binds every compiler/interpreter exhaustive
// transcript to a second, fresh executor replay. The compact Semantic IR
// replay is proof input; the full executor record retained here proves where
// its process bytes, tool checks, disposable workspaces, and cleanup came
// from.
func validateExhaustiveReplays(document Document) error {
	type expectedRecord struct {
		model    semanticir.ArtifactModel
		evidence semanticir.ExhaustiveExecutionEvidence
	}
	expected := map[string]expectedRecord{}
	for _, model := range document.SemanticIR.Artifacts {
		for _, evidence := range model.ExhaustiveEvidence {
			key := model.Artifact.ID + "\x00" + evidence.ID
			if _, duplicate := expected[key]; duplicate {
				return fmt.Errorf("certificate semantic IR repeats exhaustive execution %q", key)
			}
			expected[key] = expectedRecord{model: model, evidence: evidence}
		}
	}
	if len(expected) == 0 {
		if len(document.ExhaustiveReplays) != 0 || document.ExhaustiveReplaysSHA256 != "" {
			return errors.New("certificate carries detached exhaustive replay evidence")
		}
		return nil
	}
	if len(document.ExhaustiveReplays) != len(expected) || !validDigest(document.ExhaustiveReplaysSHA256) {
		return fmt.Errorf("certificate has %d exhaustive replays for %d semantic execution records", len(document.ExhaustiveReplays), len(expected))
	}
	aggregateDigest, err := semanticir.Digest(document.ExhaustiveReplays)
	if err != nil || aggregateDigest != document.ExhaustiveReplaysSHA256 {
		return errors.New("certificate exhaustive replay aggregate digest mismatch")
	}

	solution, ok := frozenSolutionWorkspace(document.Manifest)
	if !ok {
		return errors.New("certificate exhaustive replays have no frozen solution workspace")
	}
	workspaceID := semanticSolutionWorkspaceID(document, solution.TreeSHA256)
	seen := map[string]bool{}
	for _, replay := range document.ExhaustiveReplays {
		if err := executor.ValidateExhaustiveReplay(replay); err != nil {
			return fmt.Errorf("certificate exhaustive replay %q: %w", replay.Plan.ID, err)
		}
		key := replay.Plan.Evidence.Provenance.ArtifactID + "\x00" + replay.Plan.Evidence.ID
		want, exists := expected[key]
		if !exists || seen[key] {
			return fmt.Errorf("certificate exhaustive replay %q is detached or duplicated", replay.Plan.ID)
		}
		seen[key] = true
		taskOperations := make(map[string]semanticir.Operation, len(document.SemanticIR.Operations))
		for _, operation := range document.SemanticIR.Operations {
			taskOperations[operation.ID] = operation
		}
		seenOperations := map[string]bool{}
		for _, operation := range replay.Plan.Operations {
			if expectedOperation, exists := taskOperations[operation.ID]; !exists || !reflect.DeepEqual(operation, expectedOperation) || seenOperations[operation.ID] {
				return fmt.Errorf("certificate exhaustive replay %q has a stale, duplicate, or out-of-scope operation alphabet", replay.Plan.ID)
			}
			seenOperations[operation.ID] = true
		}
		for _, run := range replay.Plan.Evidence.Runs {
			for _, observation := range run.Observations {
				if !seenOperations[observation.Behavior.OperationID] {
					return fmt.Errorf("certificate exhaustive replay %q omits operation alphabet %q", replay.Plan.ID, observation.Behavior.OperationID)
				}
			}
		}
		planCoreDigest, err := semanticir.ExhaustiveExecutionCoreDigest(replay.Plan.Evidence)
		if err != nil {
			return fmt.Errorf("certificate exhaustive replay %q plan core: %w", replay.Plan.ID, err)
		}
		wantCoreDigest, err := semanticir.ExhaustiveExecutionCoreDigest(want.evidence)
		if err != nil || planCoreDigest != wantCoreDigest {
			return fmt.Errorf("certificate exhaustive replay %q differs from the authoritative execution core", replay.Plan.ID)
		}
		semanticReplay, err := executor.SemanticReplay(replay)
		if err != nil || !reflect.DeepEqual(semanticReplay, want.evidence.Replay) {
			return fmt.Errorf("certificate exhaustive replay %q compact transcript differs from Semantic IR", replay.Plan.ID)
		}
		if replay.Plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || replay.Plan.Workspace.TreeSHA256 != solution.TreeSHA256 || replay.Plan.Workspace.ID != workspaceID {
			return fmt.Errorf("certificate exhaustive replay %q is bound to a stale workspace", replay.Plan.ID)
		}
		if replay.Plan.Evidence.SourceDigest != want.model.Artifact.Digest {
			return fmt.Errorf("certificate exhaustive replay %q source digest differs from its artifact", replay.Plan.ID)
		}
		foundModelSource := false
		for _, source := range replay.Plan.SourceArtifacts {
			if !probeSourceFrozen(source, document) {
				return fmt.Errorf("certificate exhaustive replay %q source %q is not frozen", replay.Plan.ID, source.ID)
			}
			foundModelSource = foundModelSource || source == want.model.Artifact
		}
		if !foundModelSource {
			return fmt.Errorf("certificate exhaustive replay %q omits modeled source %q", replay.Plan.ID, want.model.Artifact.ID)
		}
		if err := requireFrozenTool(replay.Plan.Evidence.Tool, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("certificate exhaustive replay %q tool: %w", replay.Plan.ID, err)
		}
		if err := validateFrozenProbeSteps(replay.Plan.Evidence.Steps, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("certificate exhaustive replay %q steps: %w", replay.Plan.ID, err)
		}
		if err := validateFrozenProbeSteps(replay.Plan.Evidence.Replay.CleanupSteps, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("certificate exhaustive replay %q cleanup: %w", replay.Plan.ID, err)
		}
	}
	if len(seen) != len(expected) {
		return errors.New("certificate exhaustive replays truncate semantic execution evidence")
	}
	return nil
}

func validateFrozenProbeSteps(steps []semanticir.ProbeStep, frozen []taskbundle.ToolVersion) error {
	for _, step := range steps {
		if step.Tool == (semanticir.ToolRef{}) {
			if step.GeneratedExecutableID == "" {
				return fmt.Errorf("probe step %q has neither a frozen tool nor a generated executable", step.ID)
			}
			continue
		}
		if step.GeneratedExecutableID != "" {
			return fmt.Errorf("probe step %q ambiguously mixes a frozen tool and generated executable", step.ID)
		}
		if err := requireFrozenTool(step.Tool, frozen); err != nil {
			return fmt.Errorf("probe step %q: %w", step.ID, err)
		}
	}
	return nil
}

func exactTestSuiteSources(actual []semanticir.ArtifactRef, task *semanticir.Task) bool {
	if task == nil || task.Environment == nil {
		return false
	}
	want := map[semanticir.ArtifactRef]int{task.Environment.Configuration: 1}
	for _, source := range task.Environment.SourceArtifacts {
		want[source]++
	}
	for _, model := range task.Artifacts {
		if model.Kind == semanticir.ArtifactTests {
			want[model.Artifact] = 1
			if model.RunnerSelection != nil {
				want[model.RunnerSelection.Configuration] = 1
			}
		}
	}
	if len(actual) != len(want) {
		return false
	}
	for _, source := range actual {
		if want[source] != 1 {
			return false
		}
		want[source]--
	}
	return true
}

func validateSemanticTools(tools []semanticir.ToolRef, frozen []taskbundle.ToolVersion) error {
	if len(tools) != len(frozen) {
		return fmt.Errorf("certificate semantic environment binds %d tools, frozen manifest has %d", len(tools), len(frozen))
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		if seen[tool.Name] {
			return fmt.Errorf("certificate semantic environment repeats tool %q", tool.Name)
		}
		seen[tool.Name] = true
		if err := requireFrozenTool(tool, frozen); err != nil {
			return err
		}
	}
	return nil
}

func requireFrozenTool(tool semanticir.ToolRef, frozen []taskbundle.ToolVersion) error {
	for _, candidate := range frozen {
		if candidate.Name != tool.Name {
			continue
		}
		if candidate.Path != tool.Path || candidate.SHA256 != tool.Digest || candidate.ReportedVersion != tool.Version {
			return fmt.Errorf("tool %q path/digest/reported-version differs from frozen identity", tool.Name)
		}
		return nil
	}
	return fmt.Errorf("tool %q is absent from frozen environment", tool.Name)
}

func validateSemanticWorkspaces(commands []semanticir.WorkspaceCommand, frozen []taskbundle.Workspace, tools []semanticir.ToolRef) error {
	if len(commands) != len(frozen) {
		return fmt.Errorf("certificate semantic environment binds %d workspaces, frozen manifest has %d", len(commands), len(frozen))
	}
	byState := make(map[string]taskbundle.Workspace, len(frozen))
	for _, workspace := range frozen {
		state, err := semanticWorkspaceState(workspace.State)
		if err != nil {
			return err
		}
		byState[string(state)] = workspace
	}
	seenStates := make(map[string]bool, len(commands))
	for _, command := range commands {
		workspace, ok := byState[string(command.State)]
		if !ok || seenStates[string(command.State)] {
			return fmt.Errorf("semantic workspace state %q is absent or duplicated relative to the frozen manifest", command.State)
		}
		seenStates[string(command.State)] = true
		environment := semanticEnvironment(workspace.Command.Environment)
		environmentDigest, err := semanticir.Digest(environment)
		if err != nil {
			return fmt.Errorf("digest frozen workspace environment: %w", err)
		}
		if command.TreeDigest != workspace.TreeSHA256 || command.Command != workspace.Command.Text ||
			command.WorkingDirectory != workspace.Command.WorkingDirectory || command.EnvironmentDigest != environmentDigest ||
			!reflect.DeepEqual(command.Environment, environment) || !command.ClearEnvironment || !command.KillProcessGroup ||
			command.TimeoutMillis != workspace.Command.TimeoutMillis || command.ObservedPass != workspace.Result.Passed ||
			command.ExitCode != workspace.Result.ExitCode || command.StdoutDigest != workspace.Result.StdoutSHA256 ||
			command.StderrDigest != workspace.Result.StderrSHA256 || command.SignalValueDigest != workspace.Result.SignalValueSHA256 {
			return fmt.Errorf("semantic workspace state %q differs from frozen command/result evidence", command.State)
		}
		shellBound := false
		for _, tool := range tools {
			if tool.Name == workspace.Command.ShellToolName && tool.Path == workspace.Command.Shell {
				shellBound = true
				break
			}
		}
		if !shellBound {
			return fmt.Errorf("semantic workspace state %q command shell is not bound to its exact frozen ToolRef", command.State)
		}
		wantPass := workspace.State != taskbundle.BaseNewTests
		if command.ExpectedPass != wantPass || command.ObservedPass != wantPass {
			return fmt.Errorf("semantic workspace state %q violates the required pass/fail/pass triple", command.State)
		}
		if !sameToolSet(command.Tools, tools) {
			return fmt.Errorf("semantic workspace state %q truncates frozen tool bindings", command.State)
		}
		if err := validatePassSignalBinding(command.PassSignal, workspace.Command.PassSignal); err != nil {
			return fmt.Errorf("semantic workspace state %q: %w", command.State, err)
		}
	}
	return nil
}

func semanticEnvironment(environment map[string]string) []semanticir.EnvironmentVariable {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]semanticir.EnvironmentVariable, 0, len(keys))
	for _, key := range keys {
		result = append(result, semanticir.EnvironmentVariable{Name: key, Value: environment[key]})
	}
	return result
}

func semanticWorkspaceState(state taskbundle.WorkspaceState) (semanticir.WorkspaceState, error) {
	switch state {
	case taskbundle.BaseOldTests:
		return semanticir.WorkspaceBaseOldTests, nil
	case taskbundle.BaseNewTests:
		return semanticir.WorkspaceBaseNewTests, nil
	case taskbundle.SolutionNewTests:
		return semanticir.WorkspaceSolutionNewTests, nil
	default:
		return "", fmt.Errorf("unknown frozen workspace state %q", state)
	}
}

func validatePassSignalBinding(semantic semanticir.PassSignal, frozen taskbundle.PassSignal) error {
	if semantic.Expected != frozen.Expected {
		return errors.New("pass-signal expected value differs from frozen command")
	}
	switch frozen.Source {
	case taskbundle.SignalExitCode:
		if semantic.Kind != semanticir.PassSignalExitCode || semantic.Path != "" {
			return errors.New("exit-code pass signal differs from frozen command")
		}
	case taskbundle.SignalFile:
		if semantic.Kind != semanticir.PassSignalFile || semantic.Path != frozen.Path {
			return errors.New("file pass signal differs from frozen command")
		}
	default:
		return fmt.Errorf("frozen pass-signal source %q has no exact semantic representation", frozen.Source)
	}
	return nil
}

func sameToolSet(left, right []semanticir.ToolRef) bool {
	if len(left) != len(right) {
		return false
	}
	want := make(map[semanticir.ToolRef]int, len(right))
	for _, tool := range right {
		want[tool]++
	}
	for _, tool := range left {
		if want[tool] == 0 {
			return false
		}
		want[tool]--
	}
	return true
}

func workspaceEvidenceFrozen(treeDigest, environmentDigest string, commands []semanticir.WorkspaceCommand) bool {
	for _, command := range commands {
		if command.TreeDigest == treeDigest && command.EnvironmentDigest == environmentDigest {
			return true
		}
	}
	return false
}

func workspaceTreeFrozen(treeDigest string, commands []semanticir.WorkspaceCommand) bool {
	for _, command := range commands {
		if command.TreeDigest == treeDigest {
			return true
		}
	}
	return false
}

func validateTestSuiteEvidence(document Document) error {
	if document.SemanticIR.TestSuite == nil {
		return errors.New("certificate has no authoritative static TestSuite")
	}
	digest, err := semanticir.Digest(*document.SemanticIR.TestSuite)
	if err != nil || document.TestSuiteSHA256 != digest {
		return errors.New("certificate authoritative TestSuite digest mismatch")
	}
	return nil
}

// validateExecutionReport retains the mandatory fresh clean baseline even for
// a VERIFIED/vacuous run with no counterexamples. Per-witness confirmations
// remain embedded beside their witnesses; this aggregate record proves that
// the unmodified frozen solution verifier itself passed in a disposable copy.
func validateExecutionReport(document Document) error {
	digest, err := semanticir.Digest(document.ExecutionReport)
	if err != nil || digest != document.ExecutionReportSHA256 {
		return errors.New("certificate clean execution report digest mismatch")
	}
	report := document.ExecutionReport
	if err := executor.ValidateWitnessReport(&document.SemanticIR, report); err != nil {
		return fmt.Errorf("certificate typed witness report: %w", err)
	}
	if report.ReferenceAcceptance == nil {
		return errors.New("certificate lacks mandatory typed T(C) evidence")
	}
	if err := validateWitnessContextBindings(document, report.ReferenceAcceptance.Plan.Context); err != nil {
		return fmt.Errorf("certificate T(C) model bindings: %w", err)
	}
	if err := validateConfirmationExecution(document, executor.Confirmation{Command: report.ReferenceAcceptance.Command, Isolation: report.ReferenceAcceptance.Isolation}); err != nil {
		return fmt.Errorf("certificate clean baseline binding: %w", err)
	}
	wantCount := 0
	for _, counterexample := range document.Counterexamples {
		if counterexample.Obligation != ReferenceAccepted {
			wantCount++
		}
	}
	if len(report.Confirmations) != wantCount {
		return errors.New("certificate execution report confirmation count differs from proof witnesses")
	}
	want := make(map[string]executor.Confirmation, len(document.Counterexamples))
	for _, counterexample := range document.Counterexamples {
		if counterexample.Obligation != ReferenceAccepted {
			want[counterexample.ID] = counterexample.Confirmation
		}
	}
	seen := map[string]bool{}
	for _, confirmation := range report.Confirmations {
		if seen[confirmation.WitnessID] || !reflect.DeepEqual(want[confirmation.WitnessID], confirmation) {
			return fmt.Errorf("certificate execution report confirmation %q is duplicated or detached", confirmation.WitnessID)
		}
		seen[confirmation.WitnessID] = true
	}
	return nil
}

func validateWitnessContextBindings(document Document, context executor.FrozenWitnessContext) error {
	models := context.Models
	if models.SpecIRSHA256 != document.SpecIRDigest || models.ReferenceIRSHA256 != document.ReferenceIRDigest || models.TestIRSHA256 != document.TestIRDigest ||
		models.EnvironmentIRSHA256 != document.EnvironmentIRDigest || models.ProofResultSHA256 != document.ProofEvidenceSHA256 {
		return errors.New("independent Spec/reference/Test/environment/proof digests are stale or detached")
	}
	solution, ok := frozenSolutionWorkspace(document.Manifest)
	if !ok || context.Workspace.State != semanticir.WorkspaceSolutionNewTests || context.Workspace.TreeSHA256 != solution.TreeSHA256 {
		return errors.New("witness context is detached from the frozen solution workspace")
	}
	var reference, tests, environment []semanticir.ArtifactRef
	for _, model := range document.SemanticIR.Artifacts {
		switch model.Kind {
		case semanticir.ArtifactCode:
			reference = append(reference, model.Artifact)
		case semanticir.ArtifactTests:
			tests = append(tests, model.Artifact)
		}
	}
	if document.SemanticIR.Environment != nil {
		environment = append(environment, document.SemanticIR.Environment.Configuration)
		environment = append(environment, document.SemanticIR.Environment.SourceArtifacts...)
	}
	if !sameArtifactRefSet(context.ReferenceArtifacts, reference) || !sameArtifactRefSet(context.TestArtifacts, tests) || !sameArtifactRefSet(context.EnvironmentArtifacts, environment) {
		return errors.New("witness context truncates or substitutes independent source artifacts")
	}
	for _, ref := range append(append(append([]semanticir.ArtifactRef(nil), reference...), tests...), environment...) {
		if !artifactRefFrozen(ref, document.Manifest) {
			return fmt.Errorf("witness context artifact %q is not exactly frozen", ref.ID)
		}
	}
	for _, tool := range context.Tools {
		if err := requireFrozenTool(tool, document.Manifest.Environment.Tools); err != nil {
			return err
		}
	}
	return nil
}

func sameArtifactRefSet(left, right []semanticir.ArtifactRef) bool {
	if len(left) != len(right) {
		return false
	}
	want := make(map[semanticir.ArtifactRef]int, len(right))
	for _, ref := range right {
		want[ref]++
	}
	for _, ref := range left {
		if want[ref] == 0 {
			return false
		}
		want[ref]--
	}
	return true
}

func validateProbeBindings(document Document, counterexample Counterexample, confirmation executor.Confirmation) error {
	probe := confirmation.Probe
	if probe == nil {
		return fmt.Errorf("counterexample %q direct probe record is missing", counterexample.ID)
	}
	plan := probe.Plan
	wantWitness, _ := semanticir.Digest(counterexample.Witness)
	gotWitness, _ := semanticir.Digest(plan.Witness)
	if wantWitness != gotWitness || plan.WitnessID != counterexample.ID || string(plan.Obligation) != string(counterexample.Obligation) {
		return fmt.Errorf("counterexample %q direct probe is bound to a different witness/obligation", counterexample.ID)
	}
	var solution taskbundle.Workspace
	for _, workspace := range document.Manifest.Workspaces {
		if workspace.State == taskbundle.SolutionNewTests {
			solution = workspace
			break
		}
	}
	if solution.State == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || plan.Workspace.TreeSHA256 != solution.TreeSHA256 {
		return fmt.Errorf("counterexample %q direct probe is bound to a stale workspace", counterexample.ID)
	}
	workspaceID := ""
	if document.SemanticIR.Environment != nil {
		for _, command := range document.SemanticIR.Environment.Commands {
			if command.State == semanticir.WorkspaceSolutionNewTests && command.TreeDigest == solution.TreeSHA256 {
				workspaceID = command.WorkspaceID
				break
			}
		}
	}
	if workspaceID == "" || plan.Workspace.ID != workspaceID {
		return fmt.Errorf("counterexample %q direct probe workspace ID is not the frozen semantic workspace", counterexample.ID)
	}
	taskOperations := make(map[string]semanticir.Operation, len(document.SemanticIR.Operations))
	for _, operation := range document.SemanticIR.Operations {
		taskOperations[operation.ID] = operation
	}
	seenOperations := map[string]bool{}
	for _, operation := range plan.Operations {
		if expected, exists := taskOperations[operation.ID]; !exists || !reflect.DeepEqual(operation, expected) || seenOperations[operation.ID] {
			return fmt.Errorf("counterexample %q direct probe operation %q is stale, duplicated, or outside Semantic IR", counterexample.ID, operation.ID)
		}
		seenOperations[operation.ID] = true
	}
	for _, runtime := range plan.ExpectedSemantics.RuntimeOutcomes {
		if !seenOperations[runtime.Behavior.OperationID] {
			return fmt.Errorf("counterexample %q direct probe omits operation alphabet %q", counterexample.ID, runtime.Behavior.OperationID)
		}
	}
	for _, tool := range plan.Tools {
		if err := requireFrozenTool(tool, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("counterexample %q direct probe: %w", counterexample.ID, err)
		}
	}
	for _, source := range plan.SourceArtifacts {
		if !probeSourceFrozen(source, document) {
			return fmt.Errorf("counterexample %q direct probe source %q is not frozen", counterexample.ID, source.ID)
		}
	}
	return nil
}

func validateBaselineWitnessBindings(document Document, counterexample Counterexample, confirmation executor.Confirmation) error {
	evidence := confirmation.BaselineWitness
	if evidence == nil {
		return fmt.Errorf("counterexample %q baseline-witness record is missing", counterexample.ID)
	}
	plan := evidence.Plan
	wantWitness, err := semanticir.Digest(counterexample.Witness)
	if err != nil {
		return fmt.Errorf("counterexample %q baseline-witness digest: %w", counterexample.ID, err)
	}
	gotWitness, err := semanticir.Digest(plan.Witness)
	if err != nil || wantWitness != gotWitness || plan.WitnessID != counterexample.ID || string(plan.Obligation) != string(counterexample.Obligation) {
		return fmt.Errorf("counterexample %q baseline-witness evidence is bound to a different witness/obligation", counterexample.ID)
	}
	if document.SemanticIR.TestSuite == nil || plan.Vector.TestSuiteSHA256 != document.TestSuiteSHA256 {
		return fmt.Errorf("counterexample %q baseline-witness evidence is detached from the authoritative TestSuite", counterexample.ID)
	}
	predicateDigest, err := semanticir.Digest(document.SemanticIR.TestSuite.Predicate)
	if err != nil || plan.Vector.StaticPredicateSHA256 != predicateDigest {
		return fmt.Errorf("counterexample %q baseline-witness evidence is detached from the static predicate", counterexample.ID)
	}

	solution, ok := frozenSolutionWorkspace(document.Manifest)
	if !ok || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || plan.Workspace.TreeSHA256 != solution.TreeSHA256 {
		return fmt.Errorf("counterexample %q baseline-witness evidence is bound to a stale workspace", counterexample.ID)
	}
	shell, shellOK := document.Manifest.Tool(solution.Command.ShellToolName)
	wantVerifier := semanticir.ToolRef{Name: shell.Name, Path: shell.Path, Digest: shell.SHA256, Version: shell.ReportedVersion}
	if !shellOK || plan.Verifier != wantVerifier || plan.Verifier.Path != solution.Command.Shell {
		return fmt.Errorf("counterexample %q baseline verifier differs from the frozen solution command shell", counterexample.ID)
	}
	if err := requireFrozenTool(plan.Verifier, document.Manifest.Environment.Tools); err != nil {
		return fmt.Errorf("counterexample %q baseline verifier: %w", counterexample.ID, err)
	}
	workspaceID := semanticSolutionWorkspaceID(document, solution.TreeSHA256)
	if workspaceID == "" || plan.Workspace.ID != workspaceID || plan.Workspace.Root != plan.Execution.WorkspaceRoot {
		return fmt.Errorf("counterexample %q baseline-witness workspace is detached from the frozen semantic workspace", counterexample.ID)
	}
	wantExecution, err := frozenTaskEnvironment(solution, plan.Workspace.Root)
	if err != nil {
		return fmt.Errorf("counterexample %q baseline-witness execution binding: %w", counterexample.ID, err)
	}
	if !reflect.DeepEqual(plan.Execution, wantExecution) {
		return fmt.Errorf("counterexample %q baseline-witness execution differs from the frozen verifier command/environment", counterexample.ID)
	}
	if err := validateConfirmationExecution(document, confirmation); err != nil {
		return fmt.Errorf("counterexample %q baseline-witness execution: %w", counterexample.ID, err)
	}

	models := make(map[string]semanticir.ArtifactModel)
	translators := make(map[string]semanticir.ToolRef)
	for _, model := range document.SemanticIR.Artifacts {
		if model.Kind != semanticir.ArtifactCode {
			continue
		}
		models[model.Artifact.ID] = model
		translators[model.Translator.Name] = model.Translator
	}
	if len(plan.SourceArtifacts) != len(models) || len(plan.Retranslations) != len(models) || len(plan.Translators) != len(translators) {
		return fmt.Errorf("counterexample %q baseline-witness evidence truncates the frozen frontend scope", counterexample.ID)
	}
	seenSources := make(map[string]bool, len(models))
	for _, source := range plan.SourceArtifacts {
		model, found := models[source.ID]
		if !found || seenSources[source.ID] || model.Artifact != source || !probeSourceFrozen(source, document) {
			return fmt.Errorf("counterexample %q baseline-witness source %q is missing, duplicated, or stale", counterexample.ID, source.ID)
		}
		seenSources[source.ID] = true
	}
	seenModels := make(map[string]bool, len(models))
	for _, fresh := range plan.Retranslations {
		model, found := models[fresh.ArtifactID]
		freshDigest, freshErr := semanticir.ArtifactModelTranslationDigest(fresh.Model)
		modelDigest, modelErr := semanticir.ArtifactModelTranslationDigest(model)
		if !found || seenModels[fresh.ArtifactID] || freshErr != nil || modelErr != nil ||
			fresh.OriginalModelCoreSHA256 != modelDigest || fresh.FreshModelCoreSHA256 != freshDigest || freshDigest != modelDigest {
			return fmt.Errorf("counterexample %q baseline retranslation %q differs from the full certificate model", counterexample.ID, fresh.ArtifactID)
		}
		seenModels[fresh.ArtifactID] = true
	}
	seenTools := make(map[string]bool, len(translators))
	for _, tool := range plan.Translators {
		want, found := translators[tool.Name]
		if !found || seenTools[tool.Name] || tool != want {
			return fmt.Errorf("counterexample %q baseline translator %q is missing, duplicated, or detached", counterexample.ID, tool.Name)
		}
		if err := requireFrozenTool(tool, document.Manifest.Environment.Tools); err != nil {
			return fmt.Errorf("counterexample %q baseline translator: %w", counterexample.ID, err)
		}
		seenTools[tool.Name] = true
	}
	return nil
}

// validateConfirmationExecution cross-binds an executor record to the exact
// frozen solution verifier. The executor validates record self-consistency;
// this check prevents a valid record for a different command, environment, or
// workspace from being attached to the certificate.
func validateConfirmationExecution(document Document, confirmation executor.Confirmation) error {
	solution, ok := frozenSolutionWorkspace(document.Manifest)
	if !ok {
		return errors.New("frozen solution+new-tests workspace is absent")
	}
	if confirmation.Isolation == nil {
		return errors.New("fresh isolated workspace evidence is absent")
	}
	isolation := confirmation.Isolation
	if isolation.ExpectedSHA256 != solution.TreeSHA256 || isolation.OriginalBeforeSHA256 != solution.TreeSHA256 ||
		isolation.CopyBeforeSHA256 != solution.TreeSHA256 || isolation.OriginalAfterSHA256 != solution.TreeSHA256 ||
		!validDigest(isolation.CopyAfterSHA256) || isolation.OriginalRoot == "" || isolation.IsolatedRoot == "" ||
		isolation.OriginalRoot == isolation.IsolatedRoot || !filepath.IsAbs(isolation.OriginalRoot) || !filepath.IsAbs(isolation.IsolatedRoot) ||
		!isolation.IsolatedRemoved || !isolation.OriginalIntact || isolation.Error != "" {
		return errors.New("fresh isolated workspace evidence differs from the frozen solution tree")
	}
	want, err := frozenTaskEnvironment(solution, isolation.IsolatedRoot)
	if err != nil {
		return err
	}
	command := confirmation.Command
	if !reflect.DeepEqual(command.Command, want.Command) || command.WorkDir != want.WorkDir || command.Timeout != want.Timeout ||
		command.EnvironmentSHA256 != semanticir.DigestBytes([]byte(strings.Join(want.Environment, "\x00"))) {
		return errors.New("executor command/working-directory/timeout/environment differs from the frozen verifier")
	}
	wantCommandDigest, err := semanticir.Digest(want)
	if err != nil || command.CommandSHA256 != wantCommandDigest {
		return errors.New("executor command digest differs from the exact isolated frozen task environment")
	}
	if want.PassSignal.ExitCode != nil {
		if command.Signal.Kind != "exit-code" || command.Signal.ExpectedExitCode == nil || *command.Signal.ExpectedExitCode != *want.PassSignal.ExitCode {
			return errors.New("executor exit-code signal differs from the frozen verifier")
		}
	} else {
		wantFile := want.PassSignal.VerdictFile
		if wantFile == nil || command.Signal.Kind != "verdict-file" || command.Signal.VerdictPath != wantFile.Path ||
			command.Signal.ExpectedValueSHA256 != semanticir.DigestBytes([]byte(strings.TrimSpace(wantFile.PassValue))) || !command.Signal.FreshVerdict {
			return errors.New("executor verdict-file signal differs from the fresh frozen verifier signal")
		}
	}
	return nil
}

func frozenSolutionWorkspace(manifest taskbundle.Manifest) (taskbundle.Workspace, bool) {
	for _, workspace := range manifest.Workspaces {
		if workspace.State == taskbundle.SolutionNewTests {
			return workspace, true
		}
	}
	return taskbundle.Workspace{}, false
}

func semanticSolutionWorkspaceID(document Document, treeDigest string) string {
	if document.SemanticIR.Environment == nil {
		return ""
	}
	for _, command := range document.SemanticIR.Environment.Commands {
		if command.State == semanticir.WorkspaceSolutionNewTests && command.TreeDigest == treeDigest {
			return command.WorkspaceID
		}
	}
	return ""
}

func frozenTaskEnvironment(workspace taskbundle.Workspace, root string) (executor.TaskEnvironment, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return executor.TaskEnvironment{}, errors.New("executor workspace root is not an absolute canonical path")
	}
	environment := make([]string, 0, len(workspace.Command.Environment))
	for key, value := range workspace.Command.Environment {
		environment = append(environment, key+"="+value)
	}
	sort.Strings(environment)
	result := executor.TaskEnvironment{
		Command:          []string{workspace.Command.Shell, "-c", workspace.Command.Text},
		WorkspaceRoot:    root,
		WorkspaceSHA256:  workspace.TreeSHA256,
		WorkDir:          filepath.Join(root, filepath.FromSlash(workspace.Command.WorkingDirectory)),
		Timeout:          time.Duration(workspace.Command.TimeoutMillis) * time.Millisecond,
		Environment:      environment,
		ExactEnvironment: true,
	}
	signal := workspace.Command.PassSignal
	switch signal.Source {
	case taskbundle.SignalExitCode:
		if signal.Match != taskbundle.MatchExact {
			return executor.TaskEnvironment{}, errors.New("frozen exit-code signal is not exact")
		}
		code, err := strconv.Atoi(signal.Expected)
		if err != nil {
			return executor.TaskEnvironment{}, fmt.Errorf("invalid frozen exit-code signal: %w", err)
		}
		result.PassSignal = executor.ExitCodeSignal(code)
	case taskbundle.SignalFile:
		if signal.Match != taskbundle.MatchExact {
			return executor.TaskEnvironment{}, errors.New("frozen file signal is not exact and cannot certify executor confirmation")
		}
		result.PassSignal = executor.VerdictFileSignal(filepath.Join(root, filepath.FromSlash(signal.Path)), signal.Expected)
	default:
		return executor.TaskEnvironment{}, fmt.Errorf("frozen pass signal %q cannot certify executor confirmation", signal.Source)
	}
	return result, nil
}

func probeSourceFrozen(source semanticir.ArtifactRef, document Document) bool {
	for _, model := range document.SemanticIR.Artifacts {
		if model.Kind == semanticir.ArtifactCode && model.Artifact == source {
			return true
		}
	}
	if artifactRefFrozen(source, document.Manifest) {
		return true
	}
	for _, workspace := range document.Manifest.Workspaces {
		if workspace.State != taskbundle.SolutionNewTests {
			continue
		}
		for _, entry := range workspace.Entries {
			if entry.Kind == "file" && entry.Path == filepath.ToSlash(filepath.Clean(source.Path)) && entry.SHA256 == source.Digest {
				return true
			}
		}
	}
	return false
}

// artifactRefFrozen accepts either the task-root manifest path or the exact
// workspace-local path of the same frozen ID/kind/digest. Frontends must use
// workspace-local paths for execution, while certificates retain the
// task-root artifact inventory as the identity authority.
func artifactRefFrozen(ref semanticir.ArtifactRef, manifest taskbundle.Manifest) bool {
	cleanPath := filepath.ToSlash(filepath.Clean(ref.Path))
	if ref.ID == "" || ref.Path == "" || ref.Path != cleanPath || filepath.IsAbs(filepath.FromSlash(ref.Path)) || !validDigest(ref.Digest) {
		return false
	}
	identityFound := false
	for _, artifact := range manifest.Artifacts {
		if artifact.ID != ref.ID {
			continue
		}
		if artifact.Kind != string(ref.Kind) || artifact.SHA256 != ref.Digest {
			return false
		}
		identityFound = true
		if artifact.Path == cleanPath {
			return true
		}
	}
	if !identityFound {
		return false
	}
	for _, workspace := range manifest.Workspaces {
		if workspace.State != taskbundle.SolutionNewTests {
			continue
		}
		for _, entry := range workspace.Entries {
			if entry.Kind == "file" && entry.Path == cleanPath && entry.SHA256 == ref.Digest {
				return true
			}
		}
	}
	return false
}

func validateProofEvidence(document Document) error {
	result := document.ProofEvidence
	digest, err := semanticir.Digest(result)
	if err != nil {
		return fmt.Errorf("certificate typed proof evidence: %w", err)
	}
	if document.ProofEvidenceSHA256 != digest {
		return errors.New("certificate typed proof evidence digest mismatch")
	}
	if err := proofengine.ValidateResult(&document.SemanticIR, result); err != nil {
		return fmt.Errorf("certificate typed proof evidence cannot be independently reproduced: %w", err)
	}

	obligations := []proofengine.ObligationResult{result.Reference, result.FalsePositive, result.Fairness, result.ReferenceAcceptance}
	want := []semanticir.ProofObligation{
		semanticir.ObligationReferenceCorrectness,
		semanticir.ObligationTestsSound,
		semanticir.ObligationTestsComplete,
		semanticir.ObligationReferenceAcceptance,
	}
	wantAggregate := proofengine.VerdictVerified
	var witnesses []semanticir.Counterexample
	for index, obligation := range obligations {
		if obligation.Obligation != want[index] {
			return fmt.Errorf("typed proof obligation %d is %q, want %q", index, obligation.Obligation, want[index])
		}
		if obligation.Method != enumerationProofMethod && obligation.Method != smtProofMethod {
			return fmt.Errorf("typed proof obligation %q uses non-proof method %q", obligation.Obligation, obligation.Method)
		}
		if obligation.ReachableCases != result.Transcript.ReachableCases {
			return fmt.Errorf("typed proof obligation %q has inconsistent reachable-case accounting", obligation.Obligation)
		}
		switch obligation.Verdict {
		case proofengine.VerdictVerified:
			if !obligation.Exhaustive || obligation.Witness != nil || len(obligation.Blockers) != 0 {
				return fmt.Errorf("typed proof obligation %q has truncated VERIFIED evidence", obligation.Obligation)
			}
		case proofengine.VerdictNotVerified:
			if !obligation.Exhaustive || obligation.Witness == nil || len(obligation.Blockers) != 0 {
				return fmt.Errorf("typed proof obligation %q has truncated refutation evidence", obligation.Obligation)
			}
			if diagnostics := semanticir.ValidateCounterexample(&document.SemanticIR, *obligation.Witness); semanticir.HasErrors(diagnostics) {
				return fmt.Errorf("typed proof obligation %q has an invalid full-vector witness", obligation.Obligation)
			}
			witnesses = append(witnesses, *obligation.Witness)
			if wantAggregate != proofengine.VerdictProofBlocked {
				wantAggregate = proofengine.VerdictNotVerified
			}
		case proofengine.VerdictProofBlocked:
			if obligation.Exhaustive || obligation.Witness != nil || len(obligation.Blockers) == 0 {
				return fmt.Errorf("typed proof obligation %q has malformed blocker evidence", obligation.Obligation)
			}
			wantAggregate = proofengine.VerdictProofBlocked
		default:
			return fmt.Errorf("typed proof obligation %q has invalid verdict %q", obligation.Obligation, obligation.Verdict)
		}
	}
	if result.Verdict != wantAggregate {
		return fmt.Errorf("typed proof aggregate verdict %q, want %q", result.Verdict, wantAggregate)
	}
	if len(result.Counterexamples) != len(witnesses) {
		return errors.New("typed proof counterexample list truncates obligation witnesses")
	}
	for index := range witnesses {
		wantDigest, _ := semanticir.Digest(witnesses[index])
		gotDigest, _ := semanticir.Digest(result.Counterexamples[index])
		if wantDigest != gotDigest {
			return fmt.Errorf("typed proof counterexample %d differs from its obligation witness", index)
		}
	}
	if result.Verdict == proofengine.VerdictProofBlocked {
		if len(result.Blockers) == 0 || result.Transcript.Complete {
			return errors.New("typed blocked proof lacks blockers or claims a complete transcript")
		}
	} else if len(result.Blockers) != 0 || !result.Transcript.Complete {
		return errors.New("typed completed proof has blockers or an incomplete transcript")
	}

	if err := validateProofTranscript(document, result.Transcript, obligations); err != nil {
		return err
	}
	return nil
}

func validateProofTranscript(document Document, transcript proofengine.Transcript, obligations []proofengine.ObligationResult) error {
	if transcript.DomainAssignments != transcript.ExcludedAssignments+transcript.ReachableAssignments || transcript.ReachableCases != transcript.ReachableAssignments || transcript.OutcomeUniverse == 0 {
		return errors.New("typed proof transcript has inconsistent finite-universe accounting")
	}
	if transcript.Complete && transcript.Method != enumerationProofMethod && transcript.Method != smtProofMethod {
		return fmt.Errorf("typed proof transcript uses non-proof method %q", transcript.Method)
	}
	if transcript.Complete {
		for _, obligation := range obligations {
			if obligation.Method != transcript.Method {
				return fmt.Errorf("typed proof obligation %q method differs from global transcript", obligation.Obligation)
			}
		}
	}

	expectedCompiler := map[string]string{}
	for _, model := range document.SemanticIR.Artifacts {
		if model.Kind != semanticir.ArtifactCode {
			continue
		}
		for _, evidence := range model.CompilerEvidence {
			digest, _ := semanticir.Digest(evidence)
			expectedCompiler[model.Artifact.ID+"\x00"+evidence.ID] = digest
		}
	}
	if len(transcript.CompilerEvidence) != len(expectedCompiler) {
		return errors.New("typed proof transcript truncates compiler/category-partition evidence")
	}
	seenCompiler := map[string]bool{}
	for _, evidence := range transcript.CompilerEvidence {
		key := evidence.Provenance.ArtifactID + "\x00" + evidence.ID
		digest, _ := semanticir.Digest(evidence)
		if expectedCompiler[key] != digest || seenCompiler[key] {
			return fmt.Errorf("typed proof transcript has stale or duplicate compiler evidence %q", evidence.ID)
		}
		seenCompiler[key] = true
	}
	compilerDigest, err := semanticir.Digest(transcript.CompilerEvidence)
	if err != nil || transcript.CompilerEvidenceSHA256 != compilerDigest {
		return errors.New("typed proof compiler/category-partition evidence digest mismatch")
	}

	var expectedClosures []semanticir.ScopeClosureEvidence
	var expectedExhaustive []semanticir.ExhaustiveExecutionEvidence
	for _, model := range document.SemanticIR.Artifacts {
		if model.Kind == semanticir.ArtifactCode && model.ScopeClosure != nil {
			expectedClosures = append(expectedClosures, *model.ScopeClosure)
		}
		expectedExhaustive = append(expectedExhaustive, model.ExhaustiveEvidence...)
	}
	sort.Slice(expectedClosures, func(i, j int) bool {
		return expectedClosures[i].Provenance.ArtifactID < expectedClosures[j].Provenance.ArtifactID
	})
	sort.Slice(expectedExhaustive, func(i, j int) bool {
		if expectedExhaustive[i].Provenance.ArtifactID != expectedExhaustive[j].Provenance.ArtifactID {
			return expectedExhaustive[i].Provenance.ArtifactID < expectedExhaustive[j].Provenance.ArtifactID
		}
		return expectedExhaustive[i].ID < expectedExhaustive[j].ID
	})
	closureDigest, err := semanticir.Digest(transcript.ScopeClosures)
	if err != nil || !reflect.DeepEqual(transcript.ScopeClosures, expectedClosures) || transcript.ScopeClosuresSHA256 != closureDigest {
		return errors.New("typed proof scope-closure records are truncated, stale, or have a mismatched digest")
	}
	exhaustiveDigest, err := semanticir.Digest(transcript.ExhaustiveEvidence)
	if err != nil || !reflect.DeepEqual(transcript.ExhaustiveEvidence, expectedExhaustive) || transcript.ExhaustiveEvidenceSHA256 != exhaustiveDigest {
		return errors.New("typed proof exhaustive execution records are truncated, stale, or have a mismatched digest")
	}

	if transcript.TestSuite == nil {
		return errors.New("typed proof transcript truncates authoritative TestSuite evidence")
	}
	suiteDigest, err := semanticir.Digest(transcript.TestSuite)
	if err != nil || transcript.TestSuiteSHA256 != suiteDigest || document.SemanticIR.TestSuite == nil || !reflect.DeepEqual(*transcript.TestSuite, *document.SemanticIR.TestSuite) {
		return errors.New("typed proof TestSuite record/digest differs from authoritative semantic evidence")
	}

	switch transcript.Method {
	case enumerationProofMethod:
		if transcript.Solver != nil {
			return errors.New("finite-enumeration proof carries an unrelated solver transcript")
		}
	case smtProofMethod:
		if transcript.Solver == nil {
			return errors.New("SMT proof omits its solver transcript")
		}
		if err := validateSolverTranscript(*transcript.Solver, document.Manifest.Environment.Tools, obligations); err != nil {
			return err
		}
	case "":
		if transcript.Complete {
			return errors.New("complete typed proof has no method")
		}
	default:
		return fmt.Errorf("typed proof transcript uses forbidden legacy method %q", transcript.Method)
	}
	return nil
}

func validateSolverTranscript(transcript proofengine.SolverTranscript, frozen []taskbundle.ToolVersion, obligations []proofengine.ObligationResult) error {
	if transcript.Name != transcript.Tool.Name || transcript.Version != transcript.Tool.Version || transcript.Digest != transcript.Tool.Digest {
		return errors.New("typed solver transcript scalar identity differs from its exact ToolRef")
	}
	if err := requireFrozenTool(transcript.Tool, frozen); err != nil {
		return fmt.Errorf("typed solver transcript: %w", err)
	}
	if len(transcript.Queries) != len(obligations) {
		return errors.New("typed solver transcript does not contain exactly one query per obligation")
	}
	for index, query := range transcript.Queries {
		if query.Obligation != obligations[index].Obligation || query.SMTLIB == "" || query.Output == "" ||
			query.SMTLIBSHA256 != semanticir.DigestBytes([]byte(query.SMTLIB)) || query.OutputSHA256 != semanticir.DigestBytes([]byte(query.Output)) {
			return fmt.Errorf("typed solver query %d has stale formula/output evidence", index)
		}
		if firstNonemptyLine(query.Output) != query.Result || (query.Result != "sat" && query.Result != "unsat") {
			return fmt.Errorf("typed solver query %d has invalid exact result %q", index, query.Result)
		}
		if query.Result == "sat" {
			if obligations[index].Verdict != proofengine.VerdictNotVerified || query.ModelSMTLIB == "" || query.ModelOutput == "" ||
				query.ModelSMTLIBSHA256 != semanticir.DigestBytes([]byte(query.ModelSMTLIB)) || query.ModelOutputSHA256 != semanticir.DigestBytes([]byte(query.ModelOutput)) {
				return fmt.Errorf("typed SAT query %d lacks its exact model/witness evidence", index)
			}
		} else if obligations[index].Verdict != proofengine.VerdictVerified || query.ModelSMTLIB != "" || query.ModelOutput != "" || query.ModelSMTLIBSHA256 != "" || query.ModelOutputSHA256 != "" {
			return fmt.Errorf("typed UNSAT query %d has mismatched verdict or unrelated model evidence", index)
		}
	}
	return nil
}

func typedProofResult(result proofengine.Result, obligation ProofObligation) proofengine.ObligationResult {
	switch obligation {
	case ReferenceWithinSpec:
		return result.Reference
	case TestsPassWithinSpec:
		return result.FalsePositive
	case SpecWithinTestsPass:
		return result.Fairness
	case ReferenceAccepted:
		return result.ReferenceAcceptance
	default:
		return proofengine.ObligationResult{}
	}
}

func validateCommandEvidence(command executor.CommandEvidence, wantPass bool) error {
	if len(command.Command) == 0 || !validDigest(command.CommandSHA256) || strings.TrimSpace(command.WorkDir) == "" || command.Timeout <= 0 ||
		!validDigest(command.EnvironmentSHA256) || !validDigest(command.StdoutSHA256) || !validDigest(command.StderrSHA256) ||
		command.Error != "" || command.TimedOut || command.Interrupted || command.Passed != wantPass || command.StartedAt.IsZero() {
		return errors.New("missing, stale, timed-out, or mismatched command evidence")
	}
	if command.StdoutSHA256 != semanticir.DigestBytes([]byte(command.Stdout)) || command.StderrSHA256 != semanticir.DigestBytes([]byte(command.Stderr)) {
		return errors.New("command stdout/stderr content differs from its digest")
	}
	if command.Signal.Kind == "verdict-file" {
		if !command.Signal.StaleVerdictRemoved && command.Signal.FreshVerdict == false {
			return errors.New("verdict-file command lacks a fresh signal")
		}
		if !command.Signal.FreshVerdict || !validDigest(command.Signal.ExpectedValueSHA256) || !validDigest(command.Signal.ObservedValueSHA256) || !validDigest(command.SignalValueSHA256) {
			return errors.New("verdict-file command has incomplete signal digests")
		}
	} else if command.Signal.Kind != "exit-code" || command.Signal.ExpectedExitCode == nil || command.Signal.ObservedExitCode == nil {
		return errors.New("command has an invalid pass signal")
	}
	return nil
}

func behaviorEvidenceKey(operationID string, conditions semanticir.Assignment) (string, error) {
	canonical, err := semanticir.CanonicalJSON(conditions)
	if err != nil {
		return "", fmt.Errorf("canonical behavior assignment: %w", err)
	}
	return operationID + "\x00" + string(canonical), nil
}

func firstNonemptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
