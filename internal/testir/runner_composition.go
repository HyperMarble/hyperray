package testir

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// ComposeRunner builds the global conjunction selected by the exact frozen
// verifier command from independently translated compiler-backed test models.
// Stateful or ordered verifier semantics require a frontend representation and
// are rejected rather than guessed here.
func ComposeRunner(
	models []semanticir.ArtifactModel,
	sources []semanticir.ArtifactRef,
	verifier semanticir.ToolRef,
	execution semanticir.WorkspaceCommand,
	provenance semanticir.Provenance,
) (semanticir.RunnerCompositionEvidence, error) {
	ordered := append([]semanticir.ArtifactModel(nil), models...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Artifact.ID < ordered[j].Artifact.ID })
	if len(ordered) == 0 {
		return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("global runner requires at least one independently translated test artifact")
	}

	var tests []semanticir.TestModel
	var components []semanticir.RunnerCompositionComponent
	var events []semanticir.RunnerEvent
	seenTests := map[string]bool{}
	for _, model := range ordered {
		if model.Kind != semanticir.ArtifactTests || model.TestProjection == nil || model.RunnerSelection == nil {
			return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("test artifact %q lacks compiler projection or runner selection", model.Artifact.ID)
		}
		if diagnostics := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(diagnostics) {
			return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("test artifact %q is invalid: %v", model.Artifact.ID, diagnostics)
		}
		selection := *model.RunnerSelection
		if selection.Verifier != verifier || !reflect.DeepEqual(selection.Command, execution) || !selection.Complete || !selection.ConjunctivePass {
			return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("test artifact %q is not selected by the exact frozen conjunctive verifier", model.Artifact.ID)
		}
		modelDigest, err := semanticir.ArtifactModelTranslationDigest(model)
		if err != nil {
			return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("digest test artifact %q: %w", model.Artifact.ID, err)
		}
		selectionDigest, err := semanticir.RunnerSelectionDigest(selection)
		if err != nil {
			return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("digest runner selection %q: %w", model.Artifact.ID, err)
		}
		ids := append([]string(nil), selection.TestIDs...)
		sort.Strings(ids)
		components = append(components, semanticir.RunnerCompositionComponent{
			ArtifactID: model.Artifact.ID, ArtifactDigest: model.Artifact.Digest,
			ModelDigest: modelDigest, SelectionDigest: selectionDigest, TestIDs: ids,
		})
		roots := make(map[string]semanticir.TestPassRoot, len(model.TestProjection.PassRoots))
		for _, root := range model.TestProjection.PassRoots {
			roots[root.TestID] = root
		}
		modelTests := append([]semanticir.TestModel(nil), model.Tests...)
		sort.Slice(modelTests, func(i, j int) bool { return modelTests[i].ID < modelTests[j].ID })
		for _, test := range modelTests {
			if seenTests[test.ID] {
				return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("grading test ID %q is duplicated", test.ID)
			}
			seenTests[test.ID] = true
			root, exists := roots[test.ID]
			if !exists || len(root.CompilerNodeIDs) == 0 {
				return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("grading test %q lacks a compiler-derived pass root", test.ID)
			}
			events = append(events, semanticir.RunnerEvent{
				Ordinal: len(events), ID: "test::" + model.Artifact.ID + "::" + test.ID,
				Kind: semanticir.RunnerEventTest, ArtifactID: model.Artifact.ID, TestID: test.ID,
				CompilerNodeIDs: append([]string(nil), root.CompilerNodeIDs...), Provenance: test.Provenance,
			})
			tests = append(tests, test)
		}
	}

	if len(tests) == 0 {
		return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("global runner contains no translated grading tests")
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].ID < tests[j].ID })
	predicate := semanticir.StaticTestPredicate(tests, tests[0].Provenance)
	predicateDigest, err := semanticir.Digest(predicate)
	if err != nil {
		return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("digest global TestsPass predicate: %w", err)
	}
	record := semanticir.RunnerCompositionEvidence{
		Kind: semanticir.RunnerCompositionConjunction, SourceArtifacts: append([]semanticir.ArtifactRef(nil), sources...),
		Components: components, Events: events, PredicateDigest: predicateDigest,
		Verifier: verifier, Execution: execution, Complete: true, Provenance: provenance,
	}
	record.Digest, err = semanticir.RunnerCompositionDigest(record)
	if err != nil {
		return semanticir.RunnerCompositionEvidence{}, fmt.Errorf("digest global runner composition: %w", err)
	}
	return record, nil
}
