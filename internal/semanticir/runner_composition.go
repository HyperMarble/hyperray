package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

func RunnerSelectionDigest(selection RunnerSelectionEvidence) (string, error) {
	return Digest(selection)
}

// RunnerCompositionDigest excludes only the derivation envelope that points
// back to this digest. All selected sources/models/tests, order/state
// semantics, global command/pass signal, and verifier identity remain bound.
func RunnerCompositionDigest(record RunnerCompositionEvidence) (string, error) {
	return Digest(struct {
		Kind            RunnerCompositionKind        `json:"kind"`
		SourceArtifacts []ArtifactRef                `json:"source_artifacts"`
		Components      []RunnerCompositionComponent `json:"components"`
		States          []RunnerStateObject          `json:"states"`
		Events          []RunnerEvent                `json:"events"`
		PredicateDigest string                       `json:"predicate_digest"`
		Verifier        ToolRef                      `json:"verifier"`
		Execution       WorkspaceCommand             `json:"execution"`
	}{record.Kind, record.SourceArtifacts, record.Components, record.States, record.Events, record.PredicateDigest, record.Verifier, record.Execution})
}

// ValidateRunnerComposition checks the real global verifier selection and
// pass composition independently of each frontend's local analysis command.
func ValidateRunnerComposition(task *Task, suite *TestSuiteModel) []Diagnostic {
	if task == nil || suite == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "runner composition task/suite is nil", Provenance{})}
	}
	record := suite.RunnerComposition
	var diagnostics []Diagnostic
	if record.Kind != RunnerCompositionConjunction && record.Kind != RunnerCompositionOrderedStateful || !record.Complete {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "global runner composition kind/completeness is invalid", record.Provenance))
	}
	if record.Verifier != suite.Verifier || !reflect.DeepEqual(record.Execution, suite.Execution) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "global runner composition differs from authoritative verifier command/pass signal", record.Provenance))
	}
	predicateDigest, _ := Digest(suite.Predicate)
	if record.PredicateDigest != predicateDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "global runner composition predicate differs from TestsPass", record.Provenance))
	}
	if !sameArtifactSet(record.SourceArtifacts, suite.SourceArtifacts) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "global runner composition source set differs from frozen verifier sources", record.Provenance))
	}

	testArtifacts := map[string]ArtifactModel{}
	allTests := map[string]string{}
	for _, artifact := range task.Artifacts {
		if artifact.Kind != ArtifactTests {
			continue
		}
		testArtifacts[artifact.Artifact.ID] = artifact
		for _, test := range artifact.Tests {
			if previous, duplicate := allTests[test.ID]; duplicate {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, fmt.Sprintf("grading test ID %q occurs in both %q and %q", test.ID, previous, artifact.Artifact.ID), test.Provenance))
			}
			allTests[test.ID] = artifact.Artifact.ID
		}
	}
	seenComponents := map[string]struct{}{}
	for _, component := range record.Components {
		artifact, exists := testArtifacts[component.ArtifactID]
		if !exists || component.ArtifactDigest != artifact.Artifact.Digest || artifact.RunnerSelection == nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "global runner component does not bind an attached test artifact/selection", record.Provenance))
			continue
		}
		modelDigest, modelErr := ArtifactModelTranslationDigest(artifact)
		selectionDigest, selectionErr := RunnerSelectionDigest(*artifact.RunnerSelection)
		wantIDs := make([]string, 0, len(artifact.Tests))
		for _, test := range artifact.Tests {
			wantIDs = append(wantIDs, test.ID)
		}
		gotIDs := append([]string(nil), component.TestIDs...)
		sort.Strings(wantIDs)
		sort.Strings(gotIDs)
		if modelErr != nil || selectionErr != nil || component.ModelDigest != modelDigest || component.SelectionDigest != selectionDigest || !reflect.DeepEqual(gotIDs, wantIDs) || hasDuplicateStrings(gotIDs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "global runner component model/selection/test inventory is stale", record.Provenance))
		}
		if _, duplicate := seenComponents[component.ArtifactID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "global runner repeats component "+component.ArtifactID, record.Provenance))
		}
		seenComponents[component.ArtifactID] = struct{}{}
	}
	if len(seenComponents) != len(testArtifacts) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "global runner does not bind every independently translated test artifact", record.Provenance))
	}

	states := map[string]struct{}{}
	for _, state := range record.States {
		if state.ID == "" || len(state.CompilerNodeIDs) == 0 || validateProvenance(state.Provenance) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "global runner state object lacks identity/compiler binding", state.Provenance))
		}
		if state.InitialValue != nil && ValidateLiteral(*state.InitialValue) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "global runner state object has invalid initial value", state.Provenance))
		}
		if _, duplicate := states[state.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "global runner repeats state "+state.ID, state.Provenance))
		}
		states[state.ID] = struct{}{}
	}
	seenEvents, seenTests := map[string]struct{}{}, map[string]struct{}{}
	for index, event := range record.Events {
		artifact, sourceExists := testArtifacts[event.ArtifactID]
		if event.Ordinal != index || event.ID == "" || !sourceExists || len(event.CompilerNodeIDs) == 0 || validateFactSource(event.Provenance, artifact.Artifact) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "global runner event order/source/compiler binding is incomplete", event.Provenance))
		}
		if _, duplicate := seenEvents[event.ID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "global runner repeats event "+event.ID, event.Provenance))
		}
		seenEvents[event.ID] = struct{}{}
		switch event.Kind {
		case RunnerEventTest:
			if allTests[event.TestID] != event.ArtifactID {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "global runner test event selects an unknown or wrong-artifact test", event.Provenance))
			}
			if _, duplicate := seenTests[event.TestID]; duplicate {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "global runner selects test twice "+event.TestID, event.Provenance))
			}
			seenTests[event.TestID] = struct{}{}
		case RunnerEventSetup, RunnerEventTeardown:
			if event.TestID != "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "setup/teardown runner event must not masquerade as a test", event.Provenance))
			}
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "global runner event has unsupported kind", event.Provenance))
		}
		for _, stateID := range append(append([]string(nil), event.ReadsStateIDs...), event.WritesStateIDs...) {
			if _, exists := states[stateID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "global runner event refers to unknown shared state "+stateID, event.Provenance))
			}
		}
	}
	if len(seenTests) != len(allTests) || len(record.Events) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "global runner ordered events omit grading tests or are empty", record.Provenance))
	}
	if record.Kind == RunnerCompositionOrderedStateful && len(states) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "ordered-stateful runner declares no shared state objects", record.Provenance))
	}

	wantDigest, digestErr := RunnerCompositionDigest(record)
	if digestErr != nil || record.Digest != wantDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "global runner composition digest is stale", record.Provenance))
	}
	return diagnostics
}

func sameArtifactSet(left, right []ArtifactRef) bool {
	if len(left) != len(right) {
		return false
	}
	key := func(value ArtifactRef) string {
		return value.ID + "\x00" + string(value.Kind) + "\x00" + value.Path + "\x00" + value.Digest
	}
	want := map[string]struct{}{}
	for _, value := range left {
		if _, duplicate := want[key(value)]; duplicate {
			return false
		}
		want[key(value)] = struct{}{}
	}
	for _, value := range right {
		if _, exists := want[key(value)]; !exists {
			return false
		}
	}
	return true
}
