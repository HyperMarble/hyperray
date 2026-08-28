package semanticir

import (
	"fmt"
	"reflect"
	"sort"
)

const (
	ReferenceIRSchemaV1   = "ray-reference-semantic-ir/v1"
	TestIRSchemaV1        = "ray-test-semantic-ir/v1"
	EnvironmentIRSchemaV1 = "ray-environment-semantic-ir/v1"
)

// ArtifactModelTranslationDigest hashes only frontend-authored translation
// semantics. It excludes executor-attached exhaustive replay evidence and the
// wall-clock start time of otherwise exact fresh runs. All other bytes,
// digests, commands, observations, provenance, and typed model fields remain
// bound. This is the shared equality contract for baseline retranslation.
func ArtifactModelTranslationDigest(model ArtifactModel) (string, error) {
	normalized := model
	normalized.ExhaustiveEvidence = append([]ExhaustiveExecutionEvidence(nil), model.ExhaustiveEvidence...)
	for evidenceIndex := range normalized.ExhaustiveEvidence {
		evidence := &normalized.ExhaustiveEvidence[evidenceIndex]
		evidence.Replay = ExhaustiveReplayEvidence{}
		evidence.Runs = append([]ExecutionRunEvidence(nil), evidence.Runs...)
		for runIndex := range evidence.Runs {
			evidence.Runs[runIndex].StartedAtUTC = ""
		}
	}
	return Digest(normalized)
}

// CanonicalReferenceScopeDigest is the D/O interface exposed to a reference
// frontend. RequirementCase/R is deliberately absent.
func CanonicalReferenceScopeDigest(task *Task) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task is nil")
	}
	return Digest(struct {
		TaskID      string                `json:"task_id"`
		Spec        ArtifactRef           `json:"spec"`
		Domains     []Domain              `json:"domains"`
		Groundings  []AssignmentGrounding `json:"groundings"`
		Constraints []Constraint          `json:"constraints"`
		Operations  []Operation           `json:"operations"`
		Outcomes    []ObservableOutcome   `json:"outcomes"`
	}{task.ID, task.Spec, task.Domains, task.Groundings, task.Constraints, task.Operations, task.Outcomes})
}

type canonicalArtifactModelBinding struct {
	Artifact ArtifactRef `json:"artifact"`
	Digest   string      `json:"digest"`
}

func canonicalArtifactBindings(models []ArtifactModel, kind ArtifactKind) ([]canonicalArtifactModelBinding, error) {
	var result []canonicalArtifactModelBinding
	for _, model := range models {
		if model.Kind != kind {
			continue
		}
		digest, err := ArtifactModelTranslationDigest(model)
		if err != nil {
			return nil, err
		}
		result = append(result, canonicalArtifactModelBinding{model.Artifact, digest})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Artifact.ID+"\x00"+result[i].Artifact.Digest < result[j].Artifact.ID+"\x00"+result[j].Artifact.Digest
	})
	return result, nil
}

// CanonicalReferenceIRDigest binds independently translated C(x,o), its raw
// runtime facts, compiler evidence, and the D/O scope it was analyzed against.
func CanonicalReferenceIRDigest(task *Task) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task is nil")
	}
	scopeDigest, err := CanonicalReferenceScopeDigest(task)
	if err != nil {
		return "", err
	}
	models, err := canonicalArtifactBindings(task.Artifacts, ArtifactCode)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("task has no independently translated reference model")
	}
	return Digest(struct {
		Schema      string                          `json:"schema"`
		TaskID      string                          `json:"task_id"`
		ScopeDigest string                          `json:"scope_digest"`
		Models      []canonicalArtifactModelBinding `json:"models"`
		Cases       []BehaviorCase                  `json:"cases"`
	}{ReferenceIRSchemaV1, task.ID, scopeDigest, models, task.CodeCases})
}

// CanonicalTestIRDigest binds the real global T(F), every independently
// translated test model, and the D/O scope. Spec RequirementCase/R is absent.
func CanonicalTestIRDigest(task *Task) (string, error) {
	if task == nil || task.TestSuite == nil {
		return "", fmt.Errorf("task has no Test IR")
	}
	scopeDigest, err := CanonicalReferenceScopeDigest(task)
	if err != nil {
		return "", err
	}
	models, err := canonicalArtifactBindings(task.Artifacts, ArtifactTests)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("task has no independently translated test model")
	}
	return Digest(struct {
		Schema      string                          `json:"schema"`
		TaskID      string                          `json:"task_id"`
		ScopeDigest string                          `json:"scope_digest"`
		Models      []canonicalArtifactModelBinding `json:"models"`
		Suite       TestSuiteModel                  `json:"suite"`
	}{TestIRSchemaV1, task.ID, scopeDigest, models, *task.TestSuite})
}

func CanonicalEnvironmentIRDigest(task *Task) (string, error) {
	if task == nil || task.Environment == nil {
		return "", fmt.Errorf("task has no Environment IR")
	}
	return Digest(struct {
		Schema      string           `json:"schema"`
		TaskID      string           `json:"task_id"`
		Environment EnvironmentModel `json:"environment"`
	}{EnvironmentIRSchemaV1, task.ID, *task.Environment})
}

// ValidateReferenceIR is the focused independent-C validation boundary.
func ValidateReferenceIR(task *Task) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "reference IR task is nil", Provenance{})}
	}
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, ValidateSpecIRDigest(task)...)
	var flattened []BehaviorCase
	owners := map[string]int{}
	for _, model := range task.Artifacts {
		if model.Kind != ArtifactCode {
			continue
		}
		diagnostics = append(diagnostics, ValidateArtifactModel(model)...)
		request := FrontendRequest{TaskID: task.ID, Artifact: model.Artifact, Language: model.Language, Kind: model.Kind, FiniteDomains: model.Domains, Groundings: model.Groundings, Constraints: model.Constraints, Operations: model.Operations, Outcomes: model.Outcomes}
		normalized, rawDiagnostics := NormalizeReferenceCases(request, model.RawReferenceCases)
		diagnostics = append(diagnostics, rawDiagnostics...)
		if !reflect.DeepEqual(normalized, model.Cases) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "reference model cases differ from central raw-outcome normalization", model.Coverage.Provenance))
		}
		flattened = append(flattened, model.Cases...)
		for _, operation := range model.Operations {
			if operation.Kind != OperationTest {
				owners[operation.ID]++
			}
		}
	}
	if !reflect.DeepEqual(flattened, task.CodeCases) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "flattened reference cases differ from independently translated code artifacts", task.Provenance))
	}
	for _, operation := range task.Operations {
		if owners[operation.ID] != 1 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("reference operation %q has %d owners, want 1", operation.ID, owners[operation.ID]), operation.Provenance))
		}
	}
	points, pointDiagnostics := ConcreteBehaviorPoints(task)
	diagnostics = append(diagnostics, pointDiagnostics...)
	want, got := map[string]struct{}{}, map[string]struct{}{}
	for _, point := range points {
		want[BehaviorRefKey(point)] = struct{}{}
	}
	for _, item := range task.CodeCases {
		got[BehaviorCaseKey(item)] = struct{}{}
	}
	if !reflect.DeepEqual(want, got) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "reference IR does not define C for exactly every point in D", task.Provenance))
	}
	return diagnostics
}

// ValidateTestIR is the focused independent-T validation boundary.
func ValidateTestIR(task *Task) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "Test IR task is nil", Provenance{})}
	}
	reachable := map[string]struct{}{}
	for _, requirement := range task.Requirements {
		reachable[behaviorKey(requirement.OperationID, requirement.Conditions)] = struct{}{}
	}
	outcomes := map[string]map[string]struct{}{}
	for _, operation := range task.Operations {
		outcomes[operation.ID] = stringSet(operation.OutcomeIDs)
	}
	var diagnostics []Diagnostic
	for _, model := range task.Artifacts {
		if model.Kind == ArtifactTests {
			diagnostics = append(diagnostics, ValidateArtifactModel(model)...)
			diagnostics = append(diagnostics, validateTestQuantification(task, model)...)
		}
	}
	diagnostics = append(diagnostics, validateTestSuite(task, reachable, outcomes)...)
	return diagnostics
}

func ValidateEnvironmentIR(task *Task) []Diagnostic {
	if task == nil {
		return []Diagnostic{errorDiagnostic(DiagnosticInvalidInput, "Environment IR task is nil", Provenance{})}
	}
	return validateEnvironment(task.Environment)
}
