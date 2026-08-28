package pipeline

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/testir"
)

// compileTestSuite composes the one authoritative TestsPass predicate from
// independently translated test artifacts. Per-artifact compiler projection
// graphs and exact runner-selection records are mandatory; Test IR validates
// and composes their canonical graph digests. Verifier execution is not a
// source of semantic truth and is therefore absent from this stage.
func compileTestSuite(ctx context.Context, task *semanticir.Task, records []translationRecord) []string {
	if task == nil || task.Environment == nil {
		return []string{"static Test IR requires the typed frozen environment"}
	}
	execution, ok := solutionExecution(task.Environment)
	if !ok {
		return []string{"typed solution+new-tests execution evidence is absent"}
	}

	bySourceID := map[string]semanticir.ArtifactRef{}
	byEvidenceID := map[string]semanticir.Provenance{}
	addSource := func(source semanticir.ArtifactRef, provenance semanticir.Provenance) error {
		if existing, exists := bySourceID[source.ID]; exists && existing != source {
			return fmt.Errorf("static Test IR source ID %q has conflicting frozen bindings", source.ID)
		}
		bySourceID[source.ID] = source
		byEvidenceID[source.ID] = provenance
		return nil
	}
	if err := addSource(task.Environment.Configuration, task.Environment.Provenance); err != nil {
		return []string{err.Error()}
	}
	for _, source := range task.Environment.SourceArtifacts {
		provenance := semanticir.NewProvenance(source, semanticir.SourceLocation{Path: source.Path, StartLine: 1, StartColumn: 1}, semanticir.TranslationTranslated)
		if err := addSource(source, provenance); err != nil {
			return []string{err.Error()}
		}
	}

	var models []semanticir.ArtifactModel
	var verifier semanticir.ToolRef
	for _, record := range records {
		if record.model.Kind != semanticir.ArtifactTests {
			continue
		}
		if record.model.TestProjection == nil || record.model.RunnerSelection == nil {
			return []string{fmt.Sprintf("test artifact %q has no compiler-derived projection/runner evidence", record.model.Artifact.ID)}
		}
		runner := record.model.RunnerSelection
		if !runner.Complete || !runner.ConjunctivePass || !reflect.DeepEqual(runner.Command, execution) {
			return []string{fmt.Sprintf("test artifact %q runner selection differs from the exact frozen conjunctive verifier", record.model.Artifact.ID)}
		}
		if record.request.Configuration == nil || runner.Configuration != *record.request.Configuration {
			return []string{fmt.Sprintf("test artifact %q runner selection differs from its dedicated frozen runner configuration", record.model.Artifact.ID)}
		}
		if verifier.Name == "" {
			verifier = runner.Verifier
		} else if verifier != runner.Verifier {
			return []string{"test artifacts select multiple frozen verifier identities"}
		}
		models = append(models, record.model)
		configurationProvenance := semanticir.NewProvenance(*record.request.Configuration, semanticir.SourceLocation{
			Path: record.request.Configuration.Path, StartLine: 1, StartColumn: 1,
		}, semanticir.TranslationTranslated)
		if err := addSource(*record.request.Configuration, configurationProvenance); err != nil {
			return []string{err.Error()}
		}
		provenance := semanticir.NewProvenance(record.model.Artifact, semanticir.SourceLocation{
			Path: record.model.Artifact.Path, StartLine: 1, StartColumn: 1,
		}, semanticir.TranslationTranslated)
		if err := addSource(record.model.Artifact, provenance); err != nil {
			return []string{err.Error()}
		}
	}
	if len(models) == 0 || verifier.Name == "" {
		return []string{"static Test IR requires at least one complete independently translated test artifact"}
	}

	sourceIDs := make([]string, 0, len(bySourceID))
	for id := range bySourceID {
		sourceIDs = append(sourceIDs, id)
	}
	sort.Strings(sourceIDs)
	sources := make([]semanticir.ArtifactRef, 0, len(sourceIDs))
	evidence := make([]semanticir.Provenance, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		sources = append(sources, bySourceID[id])
		evidence = append(evidence, byEvidenceID[id])
	}
	runnerComposition, err := testir.ComposeRunner(models, sources, verifier, execution, task.Environment.Provenance)
	if err != nil {
		return []string{"compose exact global verifier: " + err.Error()}
	}

	suite, err := testir.CompileStatic(ctx, testir.StaticRequest{
		Task: task, TestModels: models,
		Binding: testir.SuiteBinding{
			SourceArtifacts: sources, SourceModels: models,
			Verifier: verifier, Execution: execution,
			RunnerComposition: runnerComposition,
			Provenance:        task.Environment.Provenance, Evidence: evidence,
		},
	})
	if err != nil {
		return []string{"compile authoritative static Test IR: " + err.Error()}
	}
	task.TestSuite = &suite
	return nil
}

func solutionExecution(environment *semanticir.EnvironmentModel) (semanticir.WorkspaceCommand, bool) {
	if environment == nil {
		return semanticir.WorkspaceCommand{}, false
	}
	for _, command := range environment.Commands {
		if command.State == semanticir.WorkspaceSolutionNewTests {
			return command, true
		}
	}
	return semanticir.WorkspaceCommand{}, false
}
