package testir

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/HyperMarble/hyperray/internal/executor"
	"github.com/HyperMarble/hyperray/internal/proof"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// executor.Confirm also restores carefully, but semantic retranslation must
// observe the candidate before execution. Serialize the apply/retranslate/
// execute transaction so two builders cannot race through one workspace.
var workspaceMu sync.Mutex

type preparedEdit struct {
	plan      semanticir.EditPlan
	path      string
	original  []byte
	candidate []byte
	mode      os.FileMode
}

func buildBaseline(ctx context.Context, request Request, universe *finiteUniverse, vector VectorEvidence) (models []semanticir.ArtifactModel, evidence []RetranslationEvidence, isolation SemanticIsolationEvidence, blocker *Blocker) {
	workspace, err := makeSemanticWorkspace(executorWorkspaceRoot(request.Executor), universe.workspaceSHA)
	if err != nil {
		return nil, nil, isolation, &Blocker{Stage: "semantic-isolation", Code: "semantic-isolation-failed", VectorID: vector.ID, Detail: err.Error()}
	}
	defer func() {
		closeErr := workspace.close()
		isolation = workspace.evidence
		if closeErr != nil {
			if blocker != nil {
				closeErr = fmt.Errorf("%s; prior blocker: %s", closeErr, blocker.Detail)
			}
			models, evidence = nil, nil
			blocker = &Blocker{Stage: "semantic-isolation", Code: "semantic-isolation-failed", VectorID: vector.ID, Detail: closeErr.Error()}
		}
	}()
	models, evidence, blocker = retranslateCandidate(ctx, request, vector, workspace.root, nil)
	return models, evidence, isolation, blocker
}

func buildCandidate(ctx context.Context, request Request, universe *finiteUniverse, vector VectorEvidence) (plans []semanticir.EditPlan, evidence []RetranslationEvidence, isolation SemanticIsolationEvidence, blocker *Blocker) {
	workspace, err := makeSemanticWorkspace(executorWorkspaceRoot(request.Executor), universe.workspaceSHA)
	if err != nil {
		return nil, nil, isolation, &Blocker{Stage: "semantic-isolation", Code: "semantic-isolation-failed", VectorID: vector.ID, Detail: err.Error()}
	}
	defer func() {
		closeErr := workspace.close()
		isolation = workspace.evidence
		if closeErr != nil {
			if blocker != nil {
				closeErr = fmt.Errorf("%s; prior blocker: %s", closeErr, blocker.Detail)
			}
			plans, evidence = nil, nil
			blocker = &Blocker{Stage: "semantic-isolation", Code: "semantic-isolation-failed", VectorID: vector.ID, Detail: closeErr.Error()}
		}
	}()
	expected := canonicalExpected(vector.Choices, vector.PredictedTestsPass)
	changedOwners := map[int]bool{}
	for index, choice := range vector.Choices {
		if choice.OutcomeID != universe.baseline[index].OutcomeID {
			changedOwners[universe.cases[index].owner] = true
		}
	}
	if len(changedOwners) == 0 {
		return nil, nil, isolation, &Blocker{Stage: "materialization", Code: "unexpected-noop-vector", VectorID: vector.ID, Detail: "non-baseline vector contains no changed semantic component"}
	}
	ownerIDs := make([]int, 0, len(changedOwners))
	for owner := range changedOwners {
		ownerIDs = append(ownerIDs, owner)
	}
	sort.Ints(ownerIDs)
	witness := semanticir.Counterexample{
		ID: vector.ID, Obligation: semanticir.ObligationTestsSound,
		Conditions: cloneAssignment(expected.Conditions), OperationID: expected.OperationID,
		ObservedOutcomes: append([]string(nil), expected.OutcomeIDs...), Choices: cloneChoices(vector.Choices),
		TestPasses: vector.PredictedTestsPass, Provenance: request.Task.Provenance,
	}
	plans = make([]semanticir.EditPlan, 0, len(ownerIDs))
	for _, owner := range ownerIDs {
		binding := request.Artifacts[owner]
		derivedFrontend, err := derivedFrontendRequest(binding.Frontend, executorWorkspaceRoot(request.Executor), workspace.root)
		if err != nil {
			return nil, nil, isolation, &Blocker{Stage: "materialization", Code: "candidate-workspace-invalid", VectorID: vector.ID, Detail: err.Error()}
		}
		plan, diagnostics := binding.Materialize(ctx, semanticir.MaterializationRequest{
			Frontend: derivedFrontend, Task: request.Task, Model: binding.Model, Counterexample: witness,
		})
		if semanticir.HasErrors(diagnostics) {
			return nil, nil, isolation, diagnosticBlocker("materialization", "frontend-blocked", vector.ID, diagnostics)
		}
		// Expected semantics are the Test IR builder's authoritative full-vector
		// contract. Frontends own byte edits, not proof expectations.
		plan.Expected = expected
		plan.ID = vector.ID + ":" + binding.Frontend.Artifact.ID
		if plan.WitnessID != vector.ID {
			return nil, nil, isolation, &Blocker{Stage: "materialization", Code: "wrong-witness", VectorID: vector.ID, Detail: fmt.Sprintf("frontend plan binds witness %q", plan.WitnessID)}
		}
		if plan.Artifact != binding.Frontend.Artifact {
			return nil, nil, isolation, &Blocker{Stage: "materialization", Code: "stale-artifact", VectorID: vector.ID, Detail: "frontend plan is not bound to the exact frozen owner artifact"}
		}
		if len(plan.Edits) == 0 {
			return nil, nil, isolation, &Blocker{Stage: "materialization", Code: "no-op-candidate", VectorID: vector.ID, Detail: "a non-baseline behavior vector produced no source edits"}
		}
		plans = append(plans, plan)
	}

	prepared, blocker := prepareCandidate(workspace.root, vector.ID, plans)
	if blocker != nil {
		return nil, nil, isolation, blocker
	}
	models, evidence, blocker := retranslateCandidate(ctx, request, vector, workspace.root, prepared)
	if blocker != nil {
		return nil, nil, isolation, blocker
	}
	if blocker := verifyCandidateSemantics(universe, vector, models); blocker != nil {
		return nil, nil, isolation, blocker
	}
	return plans, evidence, isolation, nil
}

func prepareCandidate(workspaceRoot, vectorID string, plans []semanticir.EditPlan) ([]preparedEdit, *Blocker) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, &Blocker{Stage: "materialization", Code: "invalid-workspace", VectorID: vectorID, Detail: err.Error()}
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, &Blocker{Stage: "materialization", Code: "invalid-workspace", VectorID: vectorID, Detail: err.Error()}
	}
	seenIDs, seenPaths := map[string]bool{}, map[string]bool{}
	prepared := make([]preparedEdit, 0, len(plans))
	for _, plan := range plans {
		if plan.ID == "" || seenIDs[plan.ID] {
			return nil, &Blocker{Stage: "materialization", Code: "duplicate-plan", VectorID: vectorID, Detail: fmt.Sprintf("empty or duplicate plan ID %q", plan.ID)}
		}
		seenIDs[plan.ID] = true
		if plan.Artifact.Kind != semanticir.ArtifactCode || !semanticir.ValidDigest(plan.Artifact.Digest) {
			return nil, &Blocker{Stage: "materialization", Code: "invalid-artifact", VectorID: vectorID, Detail: "plan has no exact frozen code artifact digest"}
		}
		if plan.Provenance.ArtifactID != plan.Artifact.ID || plan.Provenance.ArtifactDigest != plan.Artifact.Digest ||
			(plan.Provenance.Translation != semanticir.TranslationTranslated && plan.Provenance.Translation != semanticir.TranslationComplete) {
			return nil, &Blocker{Stage: "materialization", Code: "invalid-provenance", VectorID: vectorID, Detail: "plan provenance is not bound to its frozen artifact"}
		}
		path := plan.Artifact.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path = filepath.Clean(path)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, &Blocker{Stage: "materialization", Code: "invalid-artifact-path", VectorID: vectorID, Detail: fmt.Sprintf("artifact %q is unavailable or not a regular file", plan.Artifact.Path)}
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !within(root, resolved) {
			return nil, &Blocker{Stage: "materialization", Code: "artifact-outside-workspace", VectorID: vectorID, Detail: fmt.Sprintf("artifact %q escapes the verifier workspace", plan.Artifact.Path)}
		}
		path = filepath.Clean(resolved)
		if seenPaths[path] {
			return nil, &Blocker{Stage: "materialization", Code: "duplicate-artifact-plan", VectorID: vectorID, Detail: "one vector returned multiple plans for one artifact"}
		}
		seenPaths[path] = true
		original, err := os.ReadFile(path)
		if err != nil || semanticir.DigestBytes(original) != plan.Artifact.Digest {
			return nil, &Blocker{Stage: "materialization", Code: "stale-artifact", VectorID: vectorID, Detail: fmt.Sprintf("artifact %q bytes do not match its frozen digest", plan.Artifact.ID)}
		}
		edits := append([]semanticir.ByteRangeReplacement(nil), plan.Edits...)
		sort.SliceStable(edits, func(i, j int) bool {
			if edits[i].StartByte == edits[j].StartByte {
				return edits[i].EndByte < edits[j].EndByte
			}
			return edits[i].StartByte < edits[j].StartByte
		})
		for index, edit := range edits {
			if edit.StartByte < 0 || edit.EndByte < edit.StartByte || edit.EndByte > len(original) ||
				(index > 0 && (edit.StartByte < edits[index-1].EndByte || edit.StartByte == edits[index-1].StartByte)) {
				return nil, &Blocker{Stage: "materialization", Code: "invalid-edit-range", VectorID: vectorID, Detail: fmt.Sprintf("plan %q has an invalid or overlapping edit", plan.ID)}
			}
			if !bytes.Equal(original[edit.StartByte:edit.EndByte], edit.ExpectedBytes) {
				return nil, &Blocker{Stage: "materialization", Code: "stale-edit-range", VectorID: vectorID, Detail: fmt.Sprintf("plan %q expected bytes do not match", plan.ID)}
			}
		}
		candidate := applyEdits(original, edits)
		if bytes.Equal(original, candidate) {
			return nil, &Blocker{Stage: "materialization", Code: "no-op-candidate", VectorID: vectorID, Detail: fmt.Sprintf("plan %q does not change source bytes", plan.ID)}
		}
		plan.Edits = edits
		prepared = append(prepared, preparedEdit{plan: plan, path: path, original: original, candidate: candidate, mode: info.Mode().Perm()})
	}
	return prepared, nil
}

func retranslateCandidate(ctx context.Context, request Request, vector VectorEvidence, workspaceRoot string, prepared []preparedEdit) (models []semanticir.ArtifactModel, evidence []RetranslationEvidence, blocker *Blocker) {
	applied := 0
	defer func() {
		for index := applied - 1; index >= 0; index-- {
			item := prepared[index]
			if err := os.WriteFile(item.path, item.original, item.mode); err != nil {
				blocker = &Blocker{Stage: "retranslation", Code: "restore-failed", VectorID: vector.ID, Detail: err.Error()}
				models, evidence = nil, nil
				continue
			}
			observed, err := os.ReadFile(item.path)
			if err != nil || !bytes.Equal(observed, item.original) {
				blocker = &Blocker{Stage: "retranslation", Code: "restore-mismatch", VectorID: vector.ID, Detail: fmt.Sprintf("artifact for plan %q was not restored exactly", item.plan.ID)}
				models, evidence = nil, nil
			}
		}
	}()
	for index, item := range prepared {
		current, err := os.ReadFile(item.path)
		if err != nil || !bytes.Equal(current, item.original) {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "stale-before-apply", VectorID: vector.ID, Detail: fmt.Sprintf("artifact for plan %q changed before semantic revalidation", item.plan.ID)}
		}
		applied = index + 1
		if err := os.WriteFile(item.path, item.candidate, item.mode); err != nil {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "candidate-write-failed", VectorID: vector.ID, Detail: err.Error()}
		}
		observed, err := os.ReadFile(item.path)
		if err != nil || !bytes.Equal(observed, item.candidate) {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "candidate-write-mismatch", VectorID: vector.ID, Detail: fmt.Sprintf("candidate bytes for plan %q were not written exactly", item.plan.ID)}
		}
	}

	models = make([]semanticir.ArtifactModel, len(request.Artifacts))
	evidence = make([]RetranslationEvidence, len(request.Artifacts))
	for index, binding := range request.Artifacts {
		derived, err := derivedFrontendRequest(binding.Frontend, executorWorkspaceRoot(request.Executor), workspaceRoot)
		if err != nil {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "candidate-workspace-invalid", VectorID: vector.ID, Detail: err.Error()}
		}
		model, diagnostics := binding.Translate(ctx, derived)
		if semanticir.HasErrors(diagnostics) {
			return nil, nil, diagnosticBlocker("retranslation", "frontend-blocked", vector.ID, diagnostics)
		}
		if model.Artifact != derived.Artifact || model.Kind != semanticir.ArtifactCode || model.Language != binding.Model.Language || model.Translator != binding.Model.Translator {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "model-binding-mismatch", VectorID: vector.ID, Detail: fmt.Sprintf("artifact %q retranslation is not bound to the derived candidate/tool", binding.Frontend.Artifact.ID)}
		}
		if model.Coverage.Status != semanticir.TranslationComplete || len(model.Coverage.Unsupported) != 0 {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "incomplete-candidate", VectorID: vector.ID, Detail: fmt.Sprintf("artifact %q candidate translation is incomplete", binding.Frontend.Artifact.ID)}
		}
		proofDigests, proofBlocker := replayCategoryProofs(ctx, request.Task.Environment, vector.ID, model)
		if proofBlocker != nil {
			return nil, nil, proofBlocker
		}
		modelDigest, err := semanticir.Digest(model)
		if err != nil {
			return nil, nil, &Blocker{Stage: "retranslation", Code: "model-digest-failed", VectorID: vector.ID, Detail: err.Error()}
		}
		models[index] = model
		evidence[index] = RetranslationEvidence{
			ArtifactID: derived.Artifact.ID, CandidateSHA256: derived.Artifact.Digest,
			ModelSHA256: modelDigest, Model: model, Coverage: model.Coverage.Status,
			CategoryProofSHA256: proofDigests,
		}
	}

	return models, evidence, nil
}

// replayCategoryProofs turns frontend proof records back into independently
// executed evidence. Structural validation establishes that every code case
// has a whole-category realization proof and that each category partition is
// total, disjoint, and reachable/excluded as declared. Replay then reruns the
// exact frozen solver invocation. A translated witness alone is never enough.
func replayCategoryProofs(ctx context.Context, environment *semanticir.EnvironmentModel, vectorID string, model semanticir.ArtifactModel) ([]string, *Blocker) {
	diagnostics := semanticir.ValidateArtifactModel(model)
	if semanticir.HasErrors(diagnostics) {
		return nil, diagnosticBlocker("category-proof", "category-realization-unproved", vectorID, diagnostics)
	}
	if environment == nil {
		return nil, &Blocker{Stage: "category-proof", Code: "category-realization-unproved", VectorID: vectorID, Detail: "candidate has no frozen proof environment"}
	}
	items := categoryProofItems(model)
	if len(items) == 0 {
		return nil, &Blocker{Stage: "category-proof", Code: "category-realization-unproved", VectorID: vectorID, Detail: "candidate has no replayable compiler/category proofs"}
	}
	digests := make([]string, 0, len(items))
	for _, item := range items {
		if err := proof.Replay(ctx, item.proof, item.expected, environment); err != nil {
			return nil, &Blocker{Stage: "category-proof", Code: "category-proof-replay-failed", VectorID: vectorID, Detail: item.label + ": " + err.Error()}
		}
		digest, err := semanticir.Digest(item.proof)
		if err != nil {
			return nil, &Blocker{Stage: "category-proof", Code: "category-proof-digest-failed", VectorID: vectorID, Detail: err.Error()}
		}
		digests = append(digests, digest)
	}
	return digests, nil
}

type categoryProofItem struct {
	proof    semanticir.ReplayableProof
	expected semanticir.SolverResult
	label    string
}

func categoryProofItems(model semanticir.ArtifactModel) []categoryProofItem {
	var items []categoryProofItem
	for _, compilerEvidence := range model.CompilerEvidence {
		for _, partition := range compilerEvidence.Partitions {
			items = append(items,
				categoryProofItem{proof: partition.TotalityProof, expected: semanticir.SolverUNSAT, label: "partition totality"},
				categoryProofItem{proof: partition.DisjointnessProof, expected: semanticir.SolverUNSAT, label: "partition disjointness"},
			)
			for _, label := range partition.Labels {
				expected := semanticir.SolverSAT
				if label.Reachability == semanticir.ProofRefuted {
					expected = semanticir.SolverUNSAT
				}
				items = append(items, categoryProofItem{proof: label.ReachabilityProof, expected: expected, label: "label reachability"})
			}
			for _, exclusion := range partition.Exclusions {
				items = append(items, categoryProofItem{proof: exclusion.Proof, expected: semanticir.SolverUNSAT, label: "constraint exclusion"})
			}
		}
		for _, behavior := range compilerEvidence.BehaviorProofs {
			items = append(items, categoryProofItem{proof: behavior.RealizationProof, expected: semanticir.SolverUNSAT, label: "whole-category behavior realization"})
		}
	}
	return items
}

func derivedFrontendRequest(original semanticir.FrontendRequest, sourceRoot, targetRoot string) (semanticir.FrontendRequest, error) {
	request := original
	source, err := filepath.Abs(sourceRoot)
	if err != nil {
		return request, err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return request, err
	}
	workspaceRoot, err := filepath.EvalSymlinks(original.Workspace.Root)
	if err != nil || filepath.Clean(workspaceRoot) != filepath.Clean(source) {
		return request, fmt.Errorf("frontend workspace %q is not the exact executor workspace %q", original.Workspace.Root, source)
	}
	root, err := filepath.Abs(targetRoot)
	if err != nil {
		return request, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return request, err
	}
	request.Workspace.Entries = append([]semanticir.WorkspaceEntry(nil), original.Workspace.Entries...)
	refs := make(map[string]semanticir.ArtifactRef, len(request.Workspace.Entries))
	for index := range request.Workspace.Entries {
		entry := &request.Workspace.Entries[index]
		path := filepath.Join(root, filepath.Clean(entry.Path))
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !within(root, resolved) {
			return request, fmt.Errorf("workspace entry %q is unavailable or escapes the root", entry.Path)
		}
		content, err := os.ReadFile(resolved)
		if err != nil {
			return request, fmt.Errorf("read candidate workspace entry %q: %w", entry.Path, err)
		}
		entry.Artifact.Digest = semanticir.DigestBytes(content)
		entry.Provenance.ArtifactDigest = entry.Artifact.Digest
		refs[entry.Artifact.ID] = entry.Artifact
	}
	request.Workspace.Root = root
	request.Workspace.TreeDigest, err = executor.WorkspaceDigest(root)
	if err != nil {
		return request, fmt.Errorf("digest derived candidate workspace: %w", err)
	}
	request.FocusArtifacts = append([]semanticir.ArtifactRef(nil), original.FocusArtifacts...)
	for index, artifact := range request.FocusArtifacts {
		derived, ok := refs[artifact.ID]
		if !ok || derived.Path != artifact.Path || derived.Kind != artifact.Kind {
			return request, fmt.Errorf("focus artifact %q has no derived workspace entry", artifact.ID)
		}
		request.FocusArtifacts[index] = derived
	}
	artifact, ok := refs[original.Artifact.ID]
	if !ok || artifact.Path != original.Artifact.Path || artifact.Kind != original.Artifact.Kind {
		return request, fmt.Errorf("translated artifact %q has no derived workspace entry", original.Artifact.ID)
	}
	request.Artifact = artifact
	artifactPath := filepath.Join(root, filepath.Clean(artifact.Path))
	request.Source, err = os.ReadFile(artifactPath)
	if err != nil {
		return request, err
	}
	request.ChangedRanges = append([]semanticir.ChangedSourceRange(nil), original.ChangedRanges...)
	for index := range request.ChangedRanges {
		changed := &request.ChangedRanges[index]
		derived, exists := refs[changed.ArtifactID]
		if !exists || derived.Path != changed.Path {
			return request, fmt.Errorf("changed source range artifact %q has no derived workspace entry", changed.ArtifactID)
		}
		changed.Provenance.ArtifactDigest = derived.Digest
		changed.SliceDigest, err = sourceLineSliceDigest(filepath.Join(root, filepath.Clean(changed.Path)), changed.StartLine, changed.EndLine)
		if err != nil {
			return request, fmt.Errorf("digest derived changed source range %q: %w", changed.Path, err)
		}
	}
	if request.Workspace.CompilationDatabase != nil {
		derived, ok := refs[request.Workspace.CompilationDatabase.ID]
		if !ok {
			return request, fmt.Errorf("compilation database has no derived workspace entry")
		}
		request.Workspace.CompilationDatabase = &derived
	}
	return request, nil
}

func sourceLineSliceDigest(path string, startLine, endLine int) (string, error) {
	if startLine <= 0 || endLine < startLine {
		return "", fmt.Errorf("invalid line interval %d-%d", startLine, endLine)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := bytes.SplitAfter(content, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if endLine > len(lines) {
		return "", fmt.Errorf("line interval %d-%d exceeds %d lines", startLine, endLine, len(lines))
	}
	return semanticir.DigestBytes(bytes.Join(lines[startLine-1:endLine], nil)), nil
}

func verifyCandidateSemantics(universe *finiteUniverse, vector VectorEvidence, models []semanticir.ArtifactModel) *Blocker {
	for index, item := range universe.cases {
		if item.owner < 0 || item.owner >= len(models) {
			return &Blocker{Stage: "semantic-equivalence", Code: "missing-owner", VectorID: vector.ID, Detail: fmt.Sprintf("no candidate model owns %s", item.key)}
		}
		var matches []semanticir.BehaviorCase
		for _, candidateCase := range models[item.owner].Cases {
			if semanticir.BehaviorCaseKey(candidateCase) == semanticir.BehaviorRefKey(item.behavior) {
				matches = append(matches, candidateCase)
			}
		}
		if len(matches) != 1 || len(matches[0].OutcomeIDs) != 1 || matches[0].OutcomeIDs[0] != vector.Choices[index].OutcomeID {
			observed := "none or ambiguous"
			if len(matches) == 1 {
				observed = strings.Join(matches[0].OutcomeIDs, ",")
			}
			return &Blocker{Stage: "semantic-equivalence", Code: "incorrect-candidate", VectorID: vector.ID,
				Detail: fmt.Sprintf("candidate case %s translated as [%s], requested [%s]", item.key, observed, vector.Choices[index].OutcomeID)}
		}
	}
	return nil
}

func applyEdits(original []byte, edits []semanticir.ByteRangeReplacement) []byte {
	result := make([]byte, 0, len(original))
	position := 0
	for _, edit := range edits {
		result = append(result, original[position:edit.StartByte]...)
		result = append(result, edit.Replacement...)
		position = edit.EndByte
	}
	return append(result, original[position:]...)
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func diagnosticBlocker(stage, code, vectorID string, diagnostics []semanticir.Diagnostic) *Blocker {
	var messages []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == semanticir.SeverityError {
			messages = append(messages, string(diagnostic.Code)+": "+diagnostic.Message)
		}
	}
	if len(messages) == 0 {
		messages = append(messages, "frontend returned proof-blocking diagnostics")
	}
	return &Blocker{Stage: stage, Code: code, VectorID: vectorID, Detail: strings.Join(messages, "; ")}
}
