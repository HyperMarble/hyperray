package testir

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// compileStaticTests constructs TestsPass from complete, independently
// translated frozen test artifacts whose compiler dependency graphs derive
// and exactly match every TestModel predicate. Verifier executions are
// deliberately absent from this function.
func compileStaticTests(task *semanticir.Task, supplied []semanticir.ArtifactModel, workspaceRoot string) (semanticir.TestPredicate, []semanticir.ArtifactModel, []semanticir.ArtifactModelDigest, error) {
	predicate, models, digests, err := compileAttachedStaticTests(task, supplied)
	if err != nil {
		return semanticir.TestPredicate{}, nil, nil, err
	}
	for _, model := range models {
		path := model.Artifact.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspaceRoot, filepath.FromSlash(path))
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !within(workspaceRoot, resolved) {
			return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("test artifact %q is unavailable or escapes the frozen workspace", model.Artifact.ID)
		}
		content, err := os.ReadFile(resolved)
		if err != nil || semanticir.VerifyArtifact(model.Artifact, content) != nil {
			return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("test artifact %q bytes differ from its frozen model", model.Artifact.ID)
		}
	}
	return predicate, models, digests, nil
}

func compileAttachedStaticTests(task *semanticir.Task, supplied []semanticir.ArtifactModel) (semanticir.TestPredicate, []semanticir.ArtifactModel, []semanticir.ArtifactModelDigest, error) {
	if task == nil {
		return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("semantic task is nil")
	}
	wanted := map[string]semanticir.ArtifactModel{}
	for _, model := range task.Artifacts {
		if model.Kind == semanticir.ArtifactTests {
			if _, duplicate := wanted[model.Artifact.ID]; duplicate {
				return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("task repeats test artifact model %q", model.Artifact.ID)
			}
			wanted[model.Artifact.ID] = model
		}
	}
	if len(wanted) == 0 || len(supplied) != len(wanted) {
		return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("complete static test translation requires exactly %d test artifact models; got %d", len(wanted), len(supplied))
	}
	models := append([]semanticir.ArtifactModel(nil), supplied...)
	sort.Slice(models, func(i, j int) bool { return models[i].Artifact.ID < models[j].Artifact.ID })
	seen := map[string]bool{}
	for _, model := range models {
		frozen, exists := wanted[model.Artifact.ID]
		if !exists || seen[model.Artifact.ID] || model.Kind != semanticir.ArtifactTests || !reflect.DeepEqual(frozen, model) {
			return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("test artifact model %q is missing, duplicate, stale, or not attached to the semantic task", model.Artifact.ID)
		}
		seen[model.Artifact.ID] = true
	}
	predicate, digests, err := compileModelPredicates(task, models)
	if err != nil {
		return semanticir.TestPredicate{}, nil, nil, err
	}
	var modelTests []semanticir.TestModel
	for _, model := range models {
		modelTests = append(modelTests, model.Tests...)
	}
	sort.Slice(modelTests, func(i, j int) bool { return modelTests[i].ID < modelTests[j].ID })
	taskTests := append([]semanticir.TestModel(nil), task.Tests...)
	sort.Slice(taskTests, func(i, j int) bool { return taskTests[i].ID < taskTests[j].ID })
	if !reflect.DeepEqual(modelTests, taskTests) {
		return semanticir.TestPredicate{}, nil, nil, fmt.Errorf("task-level test models differ from the exact union of attached test artifacts")
	}
	return predicate, models, digests, err
}

func compileModelPredicates(task *semanticir.Task, models []semanticir.ArtifactModel) (semanticir.TestPredicate, []semanticir.ArtifactModelDigest, error) {
	if len(models) == 0 {
		return semanticir.TestPredicate{}, nil, fmt.Errorf("static TestsPass has no translated test artifact models")
	}
	var translatedTests []semanticir.TestModel
	testIDs := map[string]bool{}
	digests := make([]semanticir.ArtifactModelDigest, 0, len(models))
	previousArtifactID := ""
	for index, model := range models {
		if index > 0 && model.Artifact.ID <= previousArtifactID {
			return semanticir.TestPredicate{}, nil, fmt.Errorf("test artifact models are not uniquely sorted")
		}
		previousArtifactID = model.Artifact.ID
		if model.Coverage.Status != semanticir.TranslationComplete || len(model.Coverage.Unsupported) != 0 ||
			model.Coverage.TotalConstructs <= 0 || model.Coverage.TranslatedConstructs != model.Coverage.TotalConstructs {
			return semanticir.TestPredicate{}, nil, fmt.Errorf("test artifact model %q is not a complete translation", model.Artifact.ID)
		}
		if diagnostics := semanticir.ValidateArtifactModel(model); semanticir.HasErrors(diagnostics) {
			return semanticir.TestPredicate{}, nil, fmt.Errorf("test artifact model %q is invalid: %s", model.Artifact.ID, formatSemanticErrors(diagnostics))
		}
		// Artifact validation proves that the test dependency graph is closed,
		// but category quantification depends on the task's operation inputs and
		// semantic groundings. Keep this task-aware check at the authority seam:
		// a concrete call such as x=0 must not be promoted to the whole x>=0
		// category merely because its graph leaf names that category.
		if diagnostics := semanticir.ValidateTestObservationQuantification(task, model); semanticir.HasErrors(diagnostics) {
			return semanticir.TestPredicate{}, nil, fmt.Errorf("test artifact model %q has invalid observation quantification: %s", model.Artifact.ID, formatSemanticErrors(diagnostics))
		}
		if len(model.Tests) == 0 {
			return semanticir.TestPredicate{}, nil, fmt.Errorf("test artifact model %q has no explicit predicate (a no-test suite must translate to true)", model.Artifact.ID)
		}
		tests := append([]semanticir.TestModel(nil), model.Tests...)
		sort.Slice(tests, func(i, j int) bool { return tests[i].ID < tests[j].ID })
		for _, test := range tests {
			if test.ID == "" || testIDs[test.ID] || test.Predicate.Kind == "" {
				return semanticir.TestPredicate{}, nil, fmt.Errorf("test artifact model %q contains an empty/duplicate test or missing predicate", model.Artifact.ID)
			}
			testIDs[test.ID] = true
			translatedTests = append(translatedTests, test)
		}
		digest, err := semanticir.Digest(model)
		if err != nil {
			return semanticir.TestPredicate{}, nil, fmt.Errorf("digest test model %q: %w", model.Artifact.ID, err)
		}
		digests = append(digests, semanticir.ArtifactModelDigest{ArtifactID: model.Artifact.ID, Digest: digest})
	}
	// Compose by globally sorted TestModel ID, not artifact declaration order.
	// This is the exact conjunction used by Semantic IR and remains stable when
	// public and hidden test artifacts are presented in a different order.
	return semanticir.StaticTestPredicate(translatedTests, translatedTests[0].Provenance), digests, nil
}

func formatSemanticErrors(diagnostics []semanticir.Diagnostic) string {
	var messages []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == semanticir.SeverityError {
			messages = append(messages, string(diagnostic.Code)+": "+diagnostic.Message)
		}
	}
	if len(messages) == 0 {
		return "frontend returned proof-blocking diagnostics"
	}
	return strings.Join(messages, "; ")
}
