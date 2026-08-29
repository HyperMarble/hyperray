package testir

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// StaticRequest contains the independently translated verifier semantics.
// No candidate implementation or verifier execution can alter Predicate.
type StaticRequest struct {
	Task       *semanticir.Task
	TestModels []semanticir.ArtifactModel
	Binding    SuiteBinding
}

// CompileStatic builds the authoritative TestSuite without enumerating the
// exponential implementation-vector product. ObservationCompleteness closes
// the quotient between concrete verifier behavior and the modeled vector.
func CompileStatic(ctx context.Context, request StaticRequest) (semanticir.TestSuiteModel, error) {
	if ctx == nil {
		return semanticir.TestSuiteModel{}, fmt.Errorf("static Test IR requires a non-nil context")
	}
	predicate, models, modelDigests, err := compileAttachedStaticTests(request.Task, request.TestModels)
	if err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		return semanticir.TestSuiteModel{}, fmt.Errorf("digest static TestsPass predicate: %w", err)
	}
	sources, evidence, err := validateStaticBinding(request.Binding, models, modelDigests)
	if err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	if request.Task.Environment == nil {
		return semanticir.TestSuiteModel{}, fmt.Errorf("static Test IR requires a frozen environment")
	}
	environmentSource := false
	for _, source := range sources {
		environmentSource = environmentSource || source == request.Task.Environment.Artifact
	}
	toolFrozen := false
	for _, tool := range request.Task.Environment.Tools {
		toolFrozen = toolFrozen || tool == request.Binding.Verifier
	}
	commandFrozen := false
	for _, command := range request.Task.Environment.Commands {
		commandFrozen = commandFrozen || reflect.DeepEqual(command, request.Binding.Execution)
	}
	if !environmentSource || !toolFrozen || !commandFrozen {
		return semanticir.TestSuiteModel{}, fmt.Errorf("suite environment source, verifier, or execution is absent from the frozen task environment")
	}
	vectorCount, err := staticVectorCount(request.Task)
	if err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	observation, err := aggregateObservationCompleteness(models, modelDigests, predicateDigest, request.Binding)
	if err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	constructCount := len(observation.Constructs)
	suite := semanticir.TestSuiteModel{
		SourceArtifacts: sources, SourceModels: modelDigests, Predicate: predicate,
		Verifier: request.Binding.Verifier, Execution: request.Binding.Execution,
		VectorCount: vectorCount, RunnerComposition: request.Binding.RunnerComposition, ObservationCompleteness: observation,
		Coverage: semanticir.TranslationCoverage{
			Status: semanticir.TranslationComplete, TotalConstructs: constructCount,
			TranslatedConstructs: constructCount, Provenance: request.Binding.Provenance,
		},
		Evidence: evidence,
	}
	if diagnostics := semanticir.ValidateRunnerComposition(request.Task, &suite); semanticir.HasErrors(diagnostics) {
		return semanticir.TestSuiteModel{}, fmt.Errorf("global frozen runner composition is invalid: %s", formatSemanticErrors(diagnostics))
	}
	return suite, nil
}

func aggregateObservationCompleteness(models []semanticir.ArtifactModel, modelDigests []semanticir.ArtifactModelDigest, predicateDigest string, binding SuiteBinding) (semanticir.TestObservationCompleteness, error) {
	components := make([]semanticir.ArtifactModelDigest, 0, len(models))
	var constructs []semanticir.TestConstructEvidence
	var irKind semanticir.CompilerIRKind
	for _, model := range models {
		if model.TestProjection == nil || model.RunnerSelection == nil || !model.TestProjection.Complete || !model.RunnerSelection.Complete {
			return semanticir.TestObservationCompleteness{}, fmt.Errorf("test model %q lacks exact compiler projection/runner evidence", model.Artifact.ID)
		}
		digest, err := semanticir.TestProjectionGraphDigest(*model.TestProjection)
		if err != nil {
			return semanticir.TestObservationCompleteness{}, fmt.Errorf("digest test model %q projection graph: %w", model.Artifact.ID, err)
		}
		components = append(components, semanticir.ArtifactModelDigest{ArtifactID: model.Artifact.ID, Digest: digest})
		constructs = append(constructs, model.TestProjection.Constructs...)
		if irKind == "" {
			irKind = model.TestProjection.Derivation.IRKind
		}
	}
	sort.Slice(components, func(i, j int) bool { return components[i].ArtifactID < components[j].ArtifactID })
	sort.Slice(constructs, func(i, j int) bool {
		return constructs[i].ArtifactID+"\x00"+constructs[i].ID < constructs[j].ArtifactID+"\x00"+constructs[j].ID
	})
	for index := 1; index < len(constructs); index++ {
		if constructs[index-1].ID == constructs[index].ID {
			return semanticir.TestObservationCompleteness{}, fmt.Errorf("test projections repeat construct ID %q", constructs[index].ID)
		}
	}
	compositionDigest, err := semanticir.Digest(components)
	if err != nil {
		return semanticir.TestObservationCompleteness{}, fmt.Errorf("digest projection composition: %w", err)
	}
	harnessDigest, err := semanticir.Digest(binding.Execution)
	if err != nil {
		return semanticir.TestObservationCompleteness{}, fmt.Errorf("digest suite execution: %w", err)
	}
	return semanticir.TestObservationCompleteness{
		ProjectionComponents: components, SourceModels: append([]semanticir.ArtifactModelDigest(nil), modelDigests...),
		StaticPredicateDigest: predicateDigest, IRKind: irKind, Constructs: constructs,
		ObservationIRDigest: compositionDigest, HarnessDigest: harnessDigest, Result: semanticir.ProofProved,
		ProofDigest: compositionDigest, Provenance: binding.Provenance,
	}, nil
}

func validateStaticBinding(binding SuiteBinding, models []semanticir.ArtifactModel, modelDigests []semanticir.ArtifactModelDigest) ([]semanticir.ArtifactRef, []semanticir.Provenance, error) {
	if binding.Verifier.Name == "" || binding.Verifier.Path == "" || binding.Verifier.Version == "" || !semanticir.ValidDigest(binding.Verifier.Digest) {
		return nil, nil, fmt.Errorf("verifier tool identity is incomplete")
	}
	boundModels := append([]semanticir.ArtifactModel(nil), binding.SourceModels...)
	sort.Slice(boundModels, func(i, j int) bool { return boundModels[i].Artifact.ID < boundModels[j].Artifact.ID })
	if !reflect.DeepEqual(boundModels, models) {
		return nil, nil, fmt.Errorf("suite source models differ from complete static test translations")
	}
	sources := append([]semanticir.ArtifactRef(nil), binding.SourceArtifacts...)
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].ID == sources[j].ID {
			return sources[i].Path < sources[j].Path
		}
		return sources[i].ID < sources[j].ID
	})
	byID := map[string]semanticir.ArtifactRef{}
	for _, source := range sources {
		if source.ID == "" || source.Path == "" || !semanticir.ValidDigest(source.Digest) ||
			(source.Kind != semanticir.ArtifactTests && source.Kind != semanticir.ArtifactEnvironment && source.Kind != semanticir.ArtifactConfiguration) {
			return nil, nil, fmt.Errorf("suite source artifact %q is invalid", source.ID)
		}
		if _, duplicate := byID[source.ID]; duplicate {
			return nil, nil, fmt.Errorf("duplicate suite source artifact %q", source.ID)
		}
		byID[source.ID] = source
	}
	for index, model := range models {
		if source, exists := byID[model.Artifact.ID]; !exists || source != model.Artifact || modelDigests[index].ArtifactID != model.Artifact.ID {
			return nil, nil, fmt.Errorf("static test model %q is not exactly bound to a suite source", model.Artifact.ID)
		}
		if model.RunnerSelection == nil {
			return nil, nil, fmt.Errorf("static test model %q has no runner selection", model.Artifact.ID)
		}
		configuration := model.RunnerSelection.Configuration
		if source, exists := byID[configuration.ID]; !exists || source != configuration {
			return nil, nil, fmt.Errorf("static test model %q runner configuration is not exactly bound to a suite source", model.Artifact.ID)
		}
	}
	if err := validateStaticExecution(binding.Execution, binding.Verifier, byID); err != nil {
		return nil, nil, err
	}
	evidence := append([]semanticir.Provenance(nil), binding.Evidence...)
	if !containsProvenance(evidence, binding.Execution.Provenance) {
		evidence = append(evidence, binding.Execution.Provenance)
	}
	if !containsProvenance(evidence, binding.Provenance) {
		evidence = append(evidence, binding.Provenance)
	}
	for _, source := range sources {
		covered := false
		for _, provenance := range evidence {
			covered = covered || provenance.ArtifactID == source.ID && provenance.ArtifactDigest == source.Digest
		}
		if !covered {
			return nil, nil, fmt.Errorf("suite source artifact %q has no exact provenance evidence", source.ID)
		}
	}
	return sources, evidence, nil
}

func validateStaticExecution(command semanticir.WorkspaceCommand, verifier semanticir.ToolRef, sources map[string]semanticir.ArtifactRef) error {
	if command.ID == "" || command.WorkspaceID == "" || command.State != semanticir.WorkspaceSolutionNewTests ||
		!semanticir.ValidDigest(command.TreeDigest) || command.Command == "" || command.WorkingDirectory == "" || command.TimeoutMillis <= 0 ||
		!command.ClearEnvironment || !command.KillProcessGroup || !command.ExpectedPass || !command.ObservedPass {
		return fmt.Errorf("suite execution is not a complete frozen solution+new-tests command")
	}
	environmentDigest, err := semanticir.Digest(command.Environment)
	if err != nil || environmentDigest != command.EnvironmentDigest {
		return fmt.Errorf("suite execution exact environment digest differs from its entries")
	}
	previous := ""
	for index, variable := range command.Environment {
		if variable.Name == "" || (index > 0 && variable.Name <= previous) {
			return fmt.Errorf("suite execution environment is not uniquely sorted")
		}
		previous = variable.Name
	}
	toolFound := false
	for _, tool := range command.Tools {
		toolFound = toolFound || tool == verifier
	}
	if !toolFound {
		return fmt.Errorf("suite execution does not bind its verifier tool")
	}
	if source, exists := sources[command.Provenance.ArtifactID]; !exists || source.Digest != command.Provenance.ArtifactDigest {
		return fmt.Errorf("suite execution provenance is not bound to a declared source")
	}
	if command.PassSignal.Kind != semanticir.PassSignalExitCode && command.PassSignal.Kind != semanticir.PassSignalFile {
		return fmt.Errorf("suite execution has an invalid pass signal")
	}
	for _, digest := range []string{command.StdoutDigest, command.StderrDigest, command.SignalValueDigest} {
		if !semanticir.ValidDigest(digest) {
			return fmt.Errorf("suite execution has an invalid result digest")
		}
	}
	return nil
}

func staticVectorCount(task *semanticir.Task) (uint64, error) {
	outcomeCounts := map[string]uint64{}
	for _, operation := range task.Operations {
		outcomeCounts[operation.ID] = uint64(len(operation.OutcomeIDs))
	}
	points, diagnostics := semanticir.ConcreteBehaviorPoints(task)
	if semanticir.HasErrors(diagnostics) {
		return 0, fmt.Errorf("expand concrete behavior-point universe: %s", formatSemanticErrors(diagnostics))
	}
	if len(points) == 0 {
		return 0, fmt.Errorf("semantic task has no reachable concrete behavior points")
	}
	count := uint64(1)
	for _, point := range points {
		outcomes := outcomeCounts[point.OperationID]
		if outcomes == 0 {
			return 0, fmt.Errorf("concrete behavior point %s has no operation-local outcomes", semanticir.BehaviorRefKey(point))
		}
		if count > ^uint64(0)/outcomes {
			return 0, fmt.Errorf("candidate behavior-vector count overflows uint64")
		}
		count *= outcomes
	}
	return count, nil
}
