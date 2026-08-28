package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/executor"
	frontendcpp "github.com/HyperMarble/ray/internal/frontend/cpp"
	frontendpython "github.com/HyperMarble/ray/internal/frontend/python"
	frontendrust "github.com/HyperMarble/ray/internal/frontend/rust"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

type translationRecord struct {
	declaration      translationConfig
	request          semanticir.FrontendRequest
	model            semanticir.ArtifactModel
	exhaustiveReplay []executor.ExhaustiveReplayEvidence
	derivationReplay []executor.DerivationReplayEvidence
}

func translateArtifacts(ctx context.Context, root string, cfg config, manifest taskbundle.Manifest, configArtifact semanticir.ArtifactRef, task *semanticir.Task) ([]translationRecord, []string) {
	if task == nil {
		return nil, []string{"translation requires the strict compiled spec"}
	}
	workspace, err := solutionWorkspace(root, manifest, configArtifact)
	if err != nil {
		return nil, []string{err.Error()}
	}
	refs := make(map[string]semanticir.ArtifactRef, len(cfg.Translations))
	runnerConfigurations := make(map[string]semanticir.ArtifactRef, len(cfg.Translations))
	changedRanges := make(map[string][]semanticir.ChangedSourceRange, len(cfg.Translations))
	focus := make([]semanticir.ArtifactRef, 0, len(cfg.Translations))
	for _, declaration := range cfg.Translations {
		frozen, err := manifestArtifact(manifest, declaration.ArtifactID, semanticir.ArtifactKind(declaration.Kind))
		if err != nil {
			return nil, []string{err.Error()}
		}
		ref, err := bindWorkspaceArtifact(&workspace, frozen, declaration.WorkspacePath)
		if err != nil {
			return nil, []string{err.Error()}
		}
		refs[declaration.ArtifactID] = ref
		ranges, err := frontendChangedRanges(root, manifest, declaration, ref)
		if err != nil {
			return nil, []string{err.Error()}
		}
		changedRanges[declaration.ArtifactID] = ranges
		focus = append(focus, ref)
		if declaration.Kind == string(semanticir.ArtifactTests) {
			configuration, err := manifestArtifact(manifest, declaration.RunnerConfigurationArtifactID, semanticir.ArtifactConfiguration)
			if err != nil {
				return nil, []string{err.Error()}
			}
			configuration, err = bindUniqueWorkspaceArtifactByDigest(&workspace, configuration)
			if err != nil {
				return nil, []string{fmt.Sprintf("translation %q runner configuration: %v", declaration.ArtifactID, err)}
			}
			runnerConfigurations[declaration.ArtifactID] = configuration
		}
	}

	var records []translationRecord
	for _, declaration := range cfg.Translations {
		vocabularyScope := declaration.EntryPoints
		if declaration.Kind == string(semanticir.ArtifactTests) {
			vocabularyScope = declaration.ObservedOperations
			if len(vocabularyScope) == 0 {
				vocabularyScope = taskOperationIDs(task)
			}
		}
		operations, outcomes, err := frontendVocabulary(task, vocabularyScope)
		if err != nil {
			return nil, []string{fmt.Sprintf("translation %q: %v", declaration.ArtifactID, err)}
		}
		artifact := refs[declaration.ArtifactID]
		source, err := os.ReadFile(filepath.Join(workspace.Root, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return nil, []string{fmt.Sprintf("read translation artifact %q: %v", artifact.ID, err)}
		}
		if err := semanticir.VerifyArtifact(artifact, source); err != nil {
			return nil, []string{err.Error()}
		}
		tool, ok := manifest.Tool(declaration.ToolName)
		if !ok {
			return nil, []string{fmt.Sprintf("translation %q tool %q is absent from frozen environment", artifact.ID, declaration.ToolName)}
		}
		prover, ok := manifest.Tool(declaration.ProverToolName)
		if !ok {
			return nil, []string{fmt.Sprintf("translation %q prover tool %q is absent from frozen environment", artifact.ID, declaration.ProverToolName)}
		}
		requestWorkspace := workspace
		if declaration.CompilationDatabase != "" {
			compilationDatabase, err := workspaceEntryRef(workspace, declaration.CompilationDatabase)
			if err != nil {
				return nil, []string{fmt.Sprintf("translation %q compilation database: %v", artifact.ID, err)}
			}
			requestWorkspace.CompilationDatabase = &compilationDatabase
		}
		options := expandedOptions(declaration.Options, root, workspace.Root)
		request := semanticir.FrontendRequest{
			TaskID: cfg.TaskID, Artifact: artifact,
			Language: semanticir.Language(declaration.Language), Kind: semanticir.ArtifactKind(declaration.Kind),
			// For code, entry points select authoritative operation IDs. For
			// tests, they select test functions while Operations/Outcomes below
			// provide the independently compiled behavior vocabulary observed by
			// those functions.
			Source: source, EntryPoints: append([]string(nil), declaration.EntryPoints...),
			FiniteDomains: append([]semanticir.Domain(nil), task.Domains...),
			Groundings:    frontendGroundings(task.Groundings, operations),
			Constraints:   frontendConstraints(task.Constraints, operations),
			Operations:    operations, Outcomes: outcomes,
			Options: options, Translator: toolRef(tool), Prover: toolRef(prover),
			Workspace: requestWorkspace, FocusArtifacts: append([]semanticir.ArtifactRef(nil), focus...),
			ChangedRanges: append([]semanticir.ChangedSourceRange(nil), changedRanges[declaration.ArtifactID]...),
		}
		if request.Kind == semanticir.ArtifactTests {
			runner, ok := manifest.Tool(declaration.RunnerToolName)
			if !ok {
				return nil, []string{fmt.Sprintf("translation %q runner tool %q is absent from frozen environment", artifact.ID, declaration.RunnerToolName)}
			}
			runnerCommand, ok := solutionExecution(task.Environment)
			if !ok {
				return nil, []string{fmt.Sprintf("translation %q has no exact frozen solution+new-tests runner command", artifact.ID)}
			}
			request.Runner = toolRef(runner)
			request.RunnerCommand = &runnerCommand
			configuration := runnerConfigurations[declaration.ArtifactID]
			request.Configuration = &configuration
		}
		if diagnostics := semanticir.ValidateFrontendRequest(request); semanticir.HasErrors(diagnostics) {
			return nil, diagnosticStrings(diagnostics)
		}
		model, diagnostics := dispatchTranslate(ctx, request)
		if semanticir.HasErrors(diagnostics) {
			return nil, diagnosticStrings(diagnostics)
		}
		if model.Artifact != request.Artifact || model.Language != request.Language || model.Kind != request.Kind || model.Translator != request.Translator {
			return nil, []string{fmt.Sprintf("frontend returned evidence not bound to request for artifact %q", artifact.ID)}
		}
		derivationReplays, blockers := replayDerivationEvidence(ctx, request, &model)
		if len(blockers) != 0 {
			return nil, blockers
		}
		replays, blockers := replayExhaustiveEvidence(ctx, request, &model)
		if len(blockers) != 0 {
			return nil, blockers
		}
		if scopeDiagnostics := semanticir.ValidateArtifactScope(request, model); semanticir.HasErrors(scopeDiagnostics) {
			return nil, diagnosticStrings(scopeDiagnostics)
		}
		if validation := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(validation) {
			return nil, diagnosticStrings(validation)
		}
		if model.Kind == semanticir.ArtifactCode && len(model.Cases) == 0 {
			return nil, []string{fmt.Sprintf("code artifact %q produced no finite behavior cases", artifact.ID)}
		}
		if model.Kind == semanticir.ArtifactTests {
			if err := exactTestEntryPoints(declaration.EntryPoints, model.Tests); err != nil {
				return nil, []string{fmt.Sprintf("test artifact %q: %v", artifact.ID, err)}
			}
			for _, test := range model.Tests {
				if err := validatePredicate(test.Predicate); err != nil {
					return nil, []string{fmt.Sprintf("test artifact %q model %q: %v", artifact.ID, test.ID, err)}
				}
			}
		}
		records = append(records, translationRecord{
			declaration: declaration, request: request, model: model,
			exhaustiveReplay: replays, derivationReplay: derivationReplays,
		})
	}
	return records, nil
}

func replayDerivationEvidence(ctx context.Context, request semanticir.FrontendRequest, model *semanticir.ArtifactModel) ([]executor.DerivationReplayEvidence, []string) {
	if model == nil || len(model.CompilerEvidence) == 0 {
		return nil, nil
	}
	var records []executor.DerivationReplayEvidence
	seen := map[string]bool{}
	for _, evidence := range model.CompilerEvidence {
		if evidence.SemanticGraph == nil {
			continue
		}
		graphDigest, err := semanticir.Digest(*evidence.SemanticGraph)
		if err != nil {
			return nil, []string{fmt.Sprintf("artifact %q compiler graph %q cannot be digested: %v", model.Artifact.ID, evidence.ID, err)}
		}
		if seen[graphDigest] {
			return nil, []string{fmt.Sprintf("artifact %q repeats compiler semantic graph %s", model.Artifact.ID, graphDigest)}
		}
		seen[graphDigest] = true
		replayed := executor.ReplayDerivation(ctx, executor.DerivationReplayPlan{
			ID: model.Artifact.ID + ":derivation:" + evidence.ID,
			Workspace: executor.ProbeWorkspace{
				ID: request.Workspace.ID, Root: request.Workspace.Root,
				State: request.Workspace.State, TreeSHA256: request.Workspace.TreeDigest,
			},
			SourceArtifacts: []semanticir.ArtifactRef{model.Artifact}, Graph: *evidence.SemanticGraph,
		})
		if replayed.Status != executor.StatusConfirmed {
			return nil, derivationReplayBlockers(model.Artifact.ID, replayed)
		}
		if err := executor.ValidateDerivationReplay(replayed); err != nil {
			return nil, []string{fmt.Sprintf("artifact %q compiler derivation replay is invalid: %v", model.Artifact.ID, err)}
		}
		records = append(records, replayed)
	}
	return records, nil
}

func derivationReplayBlockers(artifactID string, replay executor.DerivationReplayEvidence) []string {
	blockers := make([]string, 0, len(replay.Blockers))
	for _, blocker := range replay.Blockers {
		message := fmt.Sprintf("artifact %q compiler derivation replay %s/%s", artifactID, blocker.Stage, blocker.Code)
		if blocker.Detail != "" {
			message += ": " + blocker.Detail
		}
		blockers = append(blockers, message)
	}
	if len(blockers) == 0 {
		blockers = append(blockers, fmt.Sprintf("artifact %q compiler derivation replay blocked without a diagnostic", artifactID))
	}
	return blockers
}

func replayExhaustiveEvidence(ctx context.Context, request semanticir.FrontendRequest, model *semanticir.ArtifactModel) ([]executor.ExhaustiveReplayEvidence, []string) {
	if model == nil || model.Kind != semanticir.ArtifactCode || len(model.ExhaustiveEvidence) == 0 {
		return nil, nil
	}
	sources := append([]semanticir.ArtifactRef(nil), request.FocusArtifacts...)
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].ID == sources[j].ID {
			return sources[i].Path < sources[j].Path
		}
		return sources[i].ID < sources[j].ID
	})
	var records []executor.ExhaustiveReplayEvidence
	for index := range model.ExhaustiveEvidence {
		evidence := model.ExhaustiveEvidence[index]
		if !reflect.DeepEqual(evidence.Replay, semanticir.ExhaustiveReplayEvidence{}) {
			return nil, []string{fmt.Sprintf("frontend artifact %q supplied caller-authored exhaustive replay evidence", model.Artifact.ID)}
		}
		replayed := executor.ReplayExhaustive(ctx, executor.ExhaustiveReplayPlan{
			ID: model.Artifact.ID + ":" + evidence.ID,
			Workspace: executor.ProbeWorkspace{
				ID: request.Workspace.ID, Root: request.Workspace.Root,
				State: request.Workspace.State, TreeSHA256: request.Workspace.TreeDigest,
			},
			SourceArtifacts: sources, Operations: append([]semanticir.Operation(nil), request.Operations...), Evidence: evidence,
		})
		if replayed.Status != executor.StatusConfirmed {
			return nil, exhaustiveReplayBlockers(model.Artifact.ID, replayed)
		}
		if err := executor.ValidateExhaustiveReplay(replayed); err != nil {
			return nil, []string{fmt.Sprintf("artifact %q exhaustive replay is invalid: %v", model.Artifact.ID, err)}
		}
		semanticReplay, err := executor.SemanticReplay(replayed)
		if err != nil {
			return nil, []string{fmt.Sprintf("artifact %q exhaustive replay cannot be attached to Semantic IR: %v", model.Artifact.ID, err)}
		}
		model.ExhaustiveEvidence[index].Replay = semanticReplay
		records = append(records, replayed)
	}
	return records, nil
}

func exhaustiveReplayBlockers(artifactID string, replay executor.ExhaustiveReplayEvidence) []string {
	blockers := make([]string, 0, len(replay.Blockers))
	for _, blocker := range replay.Blockers {
		message := fmt.Sprintf("artifact %q exhaustive replay %s/%s", artifactID, blocker.Stage, blocker.Code)
		if blocker.Detail != "" {
			message += ": " + blocker.Detail
		}
		blockers = append(blockers, message)
	}
	if len(blockers) == 0 {
		blockers = append(blockers, fmt.Sprintf("artifact %q exhaustive replay blocked without a diagnostic", artifactID))
	}
	return blockers
}

func frontendConstraints(constraints []semanticir.Constraint, operations []semanticir.Operation) []semanticir.Constraint {
	allowed := make(map[string]bool, len(operations))
	for _, operation := range operations {
		allowed[operation.ID] = true
	}
	var scoped []semanticir.Constraint
	for _, constraint := range constraints {
		if allowed[constraint.OperationID] {
			scoped = append(scoped, constraint)
		}
	}
	return scoped
}

func frontendGroundings(groundings []semanticir.AssignmentGrounding, operations []semanticir.Operation) []semanticir.AssignmentGrounding {
	allowed := make(map[string]bool, len(operations))
	for _, operation := range operations {
		allowed[operation.ID] = true
	}
	var scoped []semanticir.AssignmentGrounding
	for _, grounding := range groundings {
		if allowed[grounding.OperationID] {
			scoped = append(scoped, grounding)
		}
	}
	return scoped
}

func exactTestEntryPoints(want []string, tests []semanticir.TestModel) error {
	if len(want) == 0 {
		if len(tests) == 1 && tests[0].Predicate.Kind == semanticir.PredicateTrue && strings.HasSuffix(tests[0].ID, "#empty-suite") {
			return nil
		}
		if len(tests) == 0 {
			return nil
		}
		return fmt.Errorf("test entry_points is empty but the artifact contains translated tests")
	}
	declared := make(map[string]bool, len(want))
	for _, id := range want {
		declared[id] = true
	}
	seen := make(map[string]bool, len(tests))
	for _, test := range tests {
		if !declared[test.ID] {
			return fmt.Errorf("frontend returned undeclared test entry point %q", test.ID)
		}
		seen[test.ID] = true
	}
	for _, id := range want {
		if !seen[id] {
			return fmt.Errorf("declared test entry point %q was not translated", id)
		}
	}
	return nil
}

func taskOperationIDs(task *semanticir.Task) []string {
	ids := make([]string, 0, len(task.Operations))
	for _, operation := range task.Operations {
		if operation.Kind != semanticir.OperationTest {
			ids = append(ids, operation.ID)
		}
	}
	return ids
}

func predicateOperationIDs(predicate semanticir.TestPredicate) map[string]bool {
	ids := map[string]bool{}
	var visit func(semanticir.TestPredicate)
	visit = func(current semanticir.TestPredicate) {
		if current.Observe != nil && current.Observe.Behavior.OperationID != "" {
			ids[current.Observe.Behavior.OperationID] = true
		}
		if current.Left != nil && current.Left.OperationID != "" {
			ids[current.Left.OperationID] = true
		}
		if current.Right != nil && current.Right.OperationID != "" {
			ids[current.Right.OperationID] = true
		}
		for _, child := range current.Children {
			visit(child)
		}
	}
	visit(predicate)
	return ids
}

func frontendVocabulary(task *semanticir.Task, entryPoints []string) ([]semanticir.Operation, []semanticir.ObservableOutcome, error) {
	wanted := make(map[string]bool, len(entryPoints))
	for _, entryPoint := range entryPoints {
		wanted[entryPoint] = true
	}
	var operations []semanticir.Operation
	outcomeIDs := map[string]bool{}
	for _, operation := range task.Operations {
		if !wanted[operation.ID] {
			continue
		}
		operations = append(operations, operation)
		delete(wanted, operation.ID)
		for _, outcomeID := range operation.OutcomeIDs {
			outcomeIDs[outcomeID] = true
		}
	}
	if len(wanted) != 0 {
		missing := make([]string, 0, len(wanted))
		for entryPoint := range wanted {
			missing = append(missing, entryPoint)
		}
		sort.Strings(missing)
		return nil, nil, fmt.Errorf("entry points are absent from the strict spec: %s", strings.Join(missing, ", "))
	}
	var outcomes []semanticir.ObservableOutcome
	for _, outcome := range task.Outcomes {
		if outcomeIDs[outcome.ID] {
			outcomes = append(outcomes, outcome)
		}
	}
	return operations, outcomes, nil
}

func expandedOptions(options map[string]string, root, workspaceRoot string) map[string]string {
	expanded := cloneMap(options)
	for key, value := range expanded {
		if value == "${TASK_ROOT}" || strings.HasPrefix(value, "${TASK_ROOT}/") {
			target := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(value, "${TASK_ROOT}"), "/")))
			if relative, err := filepath.Rel(workspaceRoot, target); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				value = filepath.ToSlash(relative)
			} else {
				value = target
			}
		} else {
			value = strings.ReplaceAll(value, "${TASK_ROOT}", root)
		}
		// Keep workspace options relocatable. Exhaustive code replay and baseline
		// witness confirmation retranslate inside fresh isolated copies, so an
		// embedded original root would escape isolation or make identical
		// candidate semantics depend on a mutable path.
		value = strings.ReplaceAll(value, "${WORKSPACE_ROOT}", ".")
		expanded[key] = value
	}
	return expanded
}

func bindWorkspaceArtifact(workspace *semanticir.WorkspaceRef, frozen semanticir.ArtifactRef, workspacePath string) (semanticir.ArtifactRef, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(workspacePath)))
	for index := range workspace.Entries {
		entry := &workspace.Entries[index]
		if entry.Path != clean {
			continue
		}
		if entry.Artifact.Digest != frozen.Digest {
			return semanticir.ArtifactRef{}, fmt.Errorf("translation artifact %q digest differs between declared artifact and workspace path %q", frozen.ID, clean)
		}
		bound := semanticir.ArtifactRef{ID: frozen.ID, Kind: frozen.Kind, Path: clean, Digest: frozen.Digest}
		entry.Artifact = bound
		entry.Provenance = semanticir.NewProvenance(bound, semanticir.SourceLocation{Path: clean, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		return bound, nil
	}
	return semanticir.ArtifactRef{}, fmt.Errorf("translation artifact %q workspace_path %q is absent from the frozen solution workspace", frozen.ID, clean)
}

// bindUniqueWorkspaceArtifactByDigest binds wiring-only artifacts, such as a
// runner configuration, without inventing a second workspace-path authority.
// Ambiguous duplicate bytes block because the runner must select one exact
// regular frozen entry.
func bindUniqueWorkspaceArtifactByDigest(workspace *semanticir.WorkspaceRef, frozen semanticir.ArtifactRef) (semanticir.ArtifactRef, error) {
	match := -1
	pathMatches := make([]int, 0, 1)
	digestMatches := make([]int, 0, 1)
	for index := range workspace.Entries {
		entry := workspace.Entries[index]
		if entry.Artifact == frozen {
			return frozen, nil
		}
		if entry.Artifact.Digest != frozen.Digest {
			continue
		}
		digestMatches = append(digestMatches, index)
		if frozen.Path == entry.Path || strings.HasSuffix(filepath.ToSlash(frozen.Path), "/"+entry.Path) {
			pathMatches = append(pathMatches, index)
		}
	}
	if len(pathMatches) == 1 {
		match = pathMatches[0]
	} else if len(pathMatches) > 1 {
		return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q path suffix matches multiple solution-workspace entries", frozen.ID)
	} else if len(digestMatches) == 1 {
		match = digestMatches[0]
	} else if len(digestMatches) > 1 {
		return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q matches multiple solution-workspace entries without a unique frozen path suffix", frozen.ID)
	}
	if match == -1 {
		return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q has no same-digest solution-workspace entry", frozen.ID)
	}
	bound := semanticir.ArtifactRef{ID: frozen.ID, Kind: frozen.Kind, Path: workspace.Entries[match].Path, Digest: frozen.Digest}
	workspace.Entries[match].Artifact = bound
	workspace.Entries[match].Provenance = semanticir.NewProvenance(bound, semanticir.SourceLocation{
		Path: bound.Path, StartLine: 1, StartColumn: 1,
	}, semanticir.TranslationTranslated)
	return bound, nil
}

func workspaceEntryRef(workspace semanticir.WorkspaceRef, workspacePath string) (semanticir.ArtifactRef, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(workspacePath)))
	for _, entry := range workspace.Entries {
		if entry.Path == clean {
			return entry.Artifact, nil
		}
	}
	return semanticir.ArtifactRef{}, fmt.Errorf("workspace path %q is absent", clean)
}

func dispatchTranslate(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ArtifactModel, []semanticir.Diagnostic) {
	switch request.Language {
	case semanticir.LanguagePython:
		return frontendpython.Translate(ctx, request)
	case semanticir.LanguageRust:
		return frontendrust.Translate(ctx, request)
	case semanticir.LanguageCPP:
		return frontendcpp.Translate(ctx, request)
	default:
		return semanticir.ArtifactModel{}, []semanticir.Diagnostic{{
			Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported,
			Message: "no strict frontend for language " + string(request.Language),
		}}
	}
}

func dispatchMaterialize(ctx context.Context, request semanticir.MaterializationRequest) (semanticir.EditPlan, []semanticir.Diagnostic) {
	switch request.Frontend.Language {
	case semanticir.LanguagePython:
		return frontendpython.Materialize(ctx, request)
	case semanticir.LanguageRust:
		return frontendrust.Materialize(ctx, request)
	case semanticir.LanguageCPP:
		return frontendcpp.Materialize(ctx, request)
	default:
		return semanticir.EditPlan{}, []semanticir.Diagnostic{{
			Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported,
			Message: "no strict materializer for language " + string(request.Frontend.Language),
		}}
	}
}

func dispatchGenerateProbe(ctx context.Context, request semanticir.MaterializationRequest) (executor.ProbePlan, []semanticir.Diagnostic) {
	switch request.Frontend.Language {
	case semanticir.LanguagePython:
		return frontendpython.GenerateProbe(ctx, request)
	case semanticir.LanguageRust:
		return frontendrust.GenerateProbe(ctx, request)
	case semanticir.LanguageCPP:
		return frontendcpp.GenerateProbe(ctx, request)
	default:
		return executor.ProbePlan{}, []semanticir.Diagnostic{{
			Severity: semanticir.SeverityError, Code: semanticir.DiagnosticUnsupported,
			Message: "no strict probe generator for language " + string(request.Frontend.Language),
		}}
	}
}

func manifestArtifact(manifest taskbundle.Manifest, id string, kind semanticir.ArtifactKind) (semanticir.ArtifactRef, error) {
	for _, artifact := range manifest.Artifacts {
		if artifact.ID == id {
			if artifact.Kind != string(kind) {
				return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q has kind %q, not requested kind %q", id, artifact.Kind, kind)
			}
			return semanticir.ArtifactRef{ID: id, Kind: kind, Path: artifact.Path, Digest: artifact.SHA256}, nil
		}
	}
	return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q is absent from manifest", id)
}

func manifestArtifactByPath(manifest taskbundle.Manifest, path string, kind semanticir.ArtifactKind) (semanticir.ArtifactRef, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, artifact := range manifest.Artifacts {
		if artifact.Path == path {
			if artifact.Kind != string(kind) {
				return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact path %q has kind %q, not requested kind %q", path, artifact.Kind, kind)
			}
			return semanticir.ArtifactRef{ID: artifact.ID, Kind: kind, Path: artifact.Path, Digest: artifact.SHA256}, nil
		}
	}
	return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact path %q is absent from manifest", path)
}

func toolRef(tool taskbundle.ToolVersion) semanticir.ToolRef {
	return semanticir.ToolRef{Name: tool.Name, Path: tool.Path, Digest: tool.SHA256, Version: tool.ReportedVersion}
}

func solutionWorkspace(root string, manifest taskbundle.Manifest, provenanceArtifact semanticir.ArtifactRef) (semanticir.WorkspaceRef, error) {
	for _, workspace := range manifest.Workspaces {
		if workspace.State != taskbundle.SolutionNewTests {
			continue
		}
		workspaceRoot := filepath.Join(root, filepath.FromSlash(workspace.Path))
		workspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
		if err != nil {
			return semanticir.WorkspaceRef{}, fmt.Errorf("resolve frozen solution workspace: %w", err)
		}
		provenance := semanticir.NewProvenance(provenanceArtifact, semanticir.SourceLocation{
			Path: provenanceArtifact.Path, StartLine: 1, StartColumn: 1,
		}, semanticir.TranslationTranslated)
		exactEnvironment := semanticEnvironment(workspace.Command.Environment)
		environmentDigest, err := semanticir.Digest(exactEnvironment)
		if err != nil {
			return semanticir.WorkspaceRef{}, fmt.Errorf("digest frozen solution workspace environment: %w", err)
		}
		ref := semanticir.WorkspaceRef{
			ID: "workspace:" + string(workspace.State), State: semanticir.WorkspaceSolutionNewTests,
			Root: workspaceRoot, TreeDigest: workspace.TreeSHA256,
			WorkingDirectory: workspace.Command.WorkingDirectory,
			BuildCommand:     workspace.Command.Text, Environment: exactEnvironment, EnvironmentDigest: environmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, Provenance: provenance,
		}
		for _, entry := range workspace.Entries {
			artifact := semanticir.ArtifactRef{
				ID:   "workspace:" + string(workspace.State) + ":" + entry.Path,
				Kind: semanticir.ArtifactEnvironment, Path: entry.Path, Digest: entry.SHA256,
			}
			entryProvenance := semanticir.NewProvenance(artifact, semanticir.SourceLocation{
				Path: entry.Path, StartLine: 1, StartColumn: 1,
			}, semanticir.TranslationTranslated)
			ref.Entries = append(ref.Entries, semanticir.WorkspaceEntry{Path: entry.Path, Artifact: artifact, Provenance: entryProvenance})
		}
		return ref, nil
	}
	return semanticir.WorkspaceRef{}, fmt.Errorf("frozen solution+new-tests workspace is absent")
}

func validatePredicate(predicate semanticir.TestPredicate) error {
	switch predicate.Kind {
	case semanticir.PredicateTrue, semanticir.PredicateFalse:
		if len(predicate.Children) != 0 || predicate.Observe != nil || predicate.Left != nil || predicate.Right != nil {
			return fmt.Errorf("true predicate carries operands")
		}
	case semanticir.PredicateAnd, semanticir.PredicateOr:
		if len(predicate.Children) == 0 {
			return fmt.Errorf("%s predicate has no children", predicate.Kind)
		}
		for _, child := range predicate.Children {
			if err := validatePredicate(child); err != nil {
				return err
			}
		}
	case semanticir.PredicateNot:
		if len(predicate.Children) != 1 {
			return fmt.Errorf("not predicate requires exactly one child")
		}
		return validatePredicate(predicate.Children[0])
	case semanticir.PredicateOutcomeIn, semanticir.PredicateRaises, semanticir.PredicateHasEffect:
		if predicate.Observe == nil || predicate.Observe.Behavior.OperationID == "" {
			return fmt.Errorf("%s predicate lacks a bound observation", predicate.Kind)
		}
	case semanticir.PredicateOutcomeEqual:
		if predicate.Left == nil || predicate.Right == nil || predicate.Left.OperationID == "" || predicate.Right.OperationID == "" {
			return fmt.Errorf("outcome-equal predicate lacks both behavior references")
		}
	default:
		return fmt.Errorf("unsupported predicate kind %q", predicate.Kind)
	}
	return nil
}

func diagnosticStrings(diagnostics []semanticir.Diagnostic) []string {
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, fmt.Sprintf("%s: %s", diagnostic.Code, diagnostic.Message))
	}
	return out
}

func sameAssignment(left, right semanticir.Assignment) bool { return reflect.DeepEqual(left, right) }

func sortedEnvironment(environment map[string]string) []string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+environment[key])
	}
	return out
}
