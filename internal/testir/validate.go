package testir

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/HyperMarble/hyperray/internal/executor"
	"github.com/HyperMarble/hyperray/internal/semanticir"
)

type finiteCase struct {
	key        string
	behavior   semanticir.BehaviorRef
	outcomeIDs []string
	owner      int
}

type finiteUniverse struct {
	cases            []finiteCase
	baseline         []semanticir.BehaviorChoice
	totalVectors     uint64
	workspaceSHA     string
	staticPredicate  semanticir.TestPredicate
	testModels       []semanticir.ArtifactModel
	testModelDigests []semanticir.ArtifactModelDigest
}

func validateRequest(request Request) (*finiteUniverse, []Blocker) {
	block := func(code, detail string) (*finiteUniverse, []Blocker) {
		return nil, []Blocker{{Stage: "validation", Code: code, Detail: detail}}
	}
	if request.Task == nil {
		return block("missing-task", "authoritative semantic Task is nil")
	}
	if request.Task.ID == "" {
		return block("invalid-task", "task ID is empty")
	}
	if len(request.Artifacts) == 0 {
		return block("missing-artifacts", "at least one frozen code artifact binding is required")
	}
	staticPredicate, testModels, testModelDigests, err := compileStaticTests(request.Task, request.TestModels, executorWorkspaceRoot(request.Executor))
	if err != nil {
		return block("invalid-static-tests", err.Error())
	}
	if err := validateExecutorEnvironment(request.Executor); err != nil {
		return block("invalid-executor", err.Error())
	}
	for _, coverage := range request.Task.Coverage {
		if coverage.Status != semanticir.TranslationComplete || len(coverage.Unsupported) != 0 {
			return block("incomplete-task", "task contains incomplete or unsupported translation coverage")
		}
	}

	domains := map[string]semanticir.Domain{}
	for _, domain := range request.Task.Domains {
		if domain.ID == "" || len(domain.Values) == 0 || domains[domain.ID].ID != "" {
			return block("invalid-domain", fmt.Sprintf("domain %q is empty, duplicate, or has no values", domain.ID))
		}
		seen := map[string]bool{}
		for _, value := range domain.Values {
			if value.ID == "" || seen[value.ID] {
				return block("invalid-domain", fmt.Sprintf("domain %q contains an empty or duplicate value ID", domain.ID))
			}
			seen[value.ID] = true
		}
		domains[domain.ID] = domain
	}
	outcomeIDs := map[string]bool{}
	for _, outcome := range request.Task.Outcomes {
		if outcome.ID == "" || outcomeIDs[outcome.ID] {
			return block("invalid-outcome", fmt.Sprintf("outcome %q is empty or duplicate", outcome.ID))
		}
		outcomeIDs[outcome.ID] = true
	}
	operations := map[string]semanticir.Operation{}
	for _, operation := range request.Task.Operations {
		if operation.ID == "" || operations[operation.ID].ID != "" || len(operation.OutcomeIDs) == 0 {
			return block("invalid-operation", fmt.Sprintf("operation %q is empty, duplicate, or has no outcomes", operation.ID))
		}
		seenDomains, seenOutcomes := map[string]bool{}, map[string]bool{}
		for _, id := range operation.DomainIDs {
			if domains[id].ID == "" || seenDomains[id] {
				return block("invalid-operation", fmt.Sprintf("operation %q has missing or duplicate domain %q", operation.ID, id))
			}
			seenDomains[id] = true
		}
		for _, id := range operation.OutcomeIDs {
			if !outcomeIDs[id] || seenOutcomes[id] {
				return block("invalid-operation", fmt.Sprintf("operation %q has missing or duplicate outcome %q", operation.ID, id))
			}
			seenOutcomes[id] = true
		}
		operations[operation.ID] = operation
	}
	constraintKeys := map[string]bool{}
	for _, constraint := range request.Task.Constraints {
		operation, ok := operations[constraint.OperationID]
		if !ok || strings.TrimSpace(constraint.ID) == "" || strings.TrimSpace(constraint.Reason) == "" {
			return block("invalid-constraint", fmt.Sprintf("constraint %q has no operation, ID, or reason", constraint.ID))
		}
		if err := validateExactAssignment(constraint.Conditions, operation, domains); err != nil {
			return block("invalid-constraint", fmt.Sprintf("constraint %q: %v", constraint.ID, err))
		}
		key := behaviorKey(constraint.OperationID, constraint.Conditions)
		if constraintKeys[key] {
			return block("duplicate-constraint", fmt.Sprintf("multiple constraints exclude %s", key))
		}
		constraintKeys[key] = true
	}

	requirements := map[string][]semanticir.RequirementCase{}
	for _, requirement := range request.Task.Requirements {
		operation, ok := operations[requirement.OperationID]
		if !ok {
			return block("invalid-requirement", fmt.Sprintf("requirement %q references unknown operation %q", requirement.ID, requirement.OperationID))
		}
		if err := validateExactAssignment(requirement.Conditions, operation, domains); err != nil {
			return block("invalid-requirement", fmt.Sprintf("requirement %q: %v", requirement.ID, err))
		}
		allowed, forbidden := map[string]bool{}, map[string]bool{}
		for _, outcomeID := range requirement.RequiredOutcomes {
			if !contains(operation.OutcomeIDs, outcomeID) || allowed[outcomeID] {
				return block("invalid-requirement", fmt.Sprintf("requirement %q has missing or duplicate required outcome %q", requirement.ID, outcomeID))
			}
			allowed[outcomeID] = true
		}
		for _, outcomeID := range requirement.ForbiddenOutcomes {
			if !contains(operation.OutcomeIDs, outcomeID) || forbidden[outcomeID] || allowed[outcomeID] {
				return block("invalid-requirement", fmt.Sprintf("requirement %q has missing, duplicate, or overlapping forbidden outcome %q", requirement.ID, outcomeID))
			}
			forbidden[outcomeID] = true
		}
		if len(allowed) == 0 || len(allowed)+len(forbidden) != len(operation.OutcomeIDs) {
			return block("invalid-requirement", fmt.Sprintf("requirement %q does not partition operation %q outcomes", requirement.ID, operation.ID))
		}
		key := behaviorKey(requirement.OperationID, requirement.Conditions)
		requirements[key] = append(requirements[key], requirement)
	}

	owners := map[string]int{}
	artifactIDs := map[string]bool{}
	workspaceSHA := ""
	for index, binding := range request.Artifacts {
		if binding.Translate == nil || binding.Materialize == nil {
			return block("missing-frontend", fmt.Sprintf("artifact binding %d lacks Translate or Materialize callback", index))
		}
		if binding.Frontend.Kind != semanticir.ArtifactCode || binding.Model.Kind != semanticir.ArtifactCode || binding.Model.Artifact != binding.Frontend.Artifact {
			return block("invalid-artifact-binding", fmt.Sprintf("binding %d does not exactly pair one frozen code artifact and model", index))
		}
		if artifactIDs[binding.Frontend.Artifact.ID] {
			return block("invalid-artifact-binding", fmt.Sprintf("duplicate code artifact binding %q", binding.Frontend.Artifact.ID))
		}
		artifactIDs[binding.Frontend.Artifact.ID] = true
		if binding.Model.Coverage.Status != semanticir.TranslationComplete || len(binding.Model.Coverage.Unsupported) != 0 {
			return block("incomplete-artifact", fmt.Sprintf("artifact %q translation is not complete", binding.Frontend.Artifact.ID))
		}
		if err := validateFrozenBinding(executorWorkspaceRoot(request.Executor), binding); err != nil {
			return block("stale-artifact-binding", fmt.Sprintf("artifact %q: %v", binding.Frontend.Artifact.ID, err))
		}
		if workspaceSHA == "" {
			workspaceSHA = binding.Frontend.Workspace.TreeDigest
		} else if workspaceSHA != binding.Frontend.Workspace.TreeDigest {
			return block("workspace-mismatch", "code frontends do not share one exact frozen workspace tree")
		}
		for _, operation := range binding.Model.Operations {
			if operation.Kind == semanticir.OperationTest {
				continue
			}
			if prior, exists := owners[operation.ID]; exists && prior != index {
				return block("ambiguous-owner", fmt.Sprintf("operation %q is owned by multiple code artifacts", operation.ID))
			}
			owners[operation.ID] = index
		}
	}

	points, pointDiagnostics := semanticir.ConcreteBehaviorPoints(request.Task)
	if semanticir.HasErrors(pointDiagnostics) {
		return block("non-finite-concrete-category", formatSemanticErrors(pointDiagnostics))
	}
	var cases []finiteCase
	usedRequirements := map[string]bool{}
	for _, point := range points {
		operation := operations[point.OperationID]
		owner, ok := owners[point.OperationID]
		if !ok {
			return block("missing-owner", fmt.Sprintf("operation %q has no materializing code artifact", point.OperationID))
		}
		categoryKey := behaviorKey(point.OperationID, point.Conditions)
		matchingRequirements, exists := requirements[categoryKey]
		if !exists || len(matchingRequirements) == 0 {
			return block("missing-requirement", fmt.Sprintf("reachable concrete point %s has no exact category requirement", semanticir.BehaviorRefKey(point)))
		}
		usedRequirements[categoryKey] = true
		outcomes := append([]string(nil), operation.OutcomeIDs...)
		sort.Strings(outcomes)
		point.Provenance = matchingRequirements[0].Provenance
		cases = append(cases, finiteCase{key: semanticir.BehaviorRefKey(point), behavior: cloneBehavior(point), outcomeIDs: outcomes, owner: owner})
	}
	if len(usedRequirements) != len(requirements) {
		keys := make([]string, 0, len(requirements))
		for key := range requirements {
			if !usedRequirements[key] {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		return block("unreachable-requirement", "requirements describe excluded or undeclared cases: "+strings.Join(keys, ", "))
	}
	if len(cases) == 0 {
		return block("empty-universe", "task has no reachable semantic behavior cases")
	}

	baseline := make([]semanticir.BehaviorChoice, len(cases))
	for index, item := range cases {
		var matches []semanticir.BehaviorCase
		for _, codeCase := range request.Artifacts[item.owner].Model.Cases {
			if semanticir.BehaviorCaseKey(codeCase) == semanticir.BehaviorRefKey(item.behavior) {
				matches = append(matches, codeCase)
			}
		}
		if len(matches) != 1 || len(matches[0].OutcomeIDs) != 1 {
			return block("non-exact-reference", fmt.Sprintf("frozen code must lower %s to exactly one outcome", item.key))
		}
		if !contains(item.outcomeIDs, matches[0].OutcomeIDs[0]) {
			return block("invalid-reference-outcome", fmt.Sprintf("frozen code selects outcome %q outside %s", matches[0].OutcomeIDs[0], item.key))
		}
		baseline[index] = semanticir.BehaviorChoice{Behavior: cloneBehavior(item.behavior), OutcomeID: matches[0].OutcomeIDs[0]}
	}
	total, err := totalVectorCount(cases)
	if err != nil {
		return block("vector-overflow", err.Error())
	}
	if request.Executor.WorkspaceSHA256 != workspaceSHA {
		return block("workspace-mismatch", "executor workspace digest differs from the frozen frontend tree")
	}
	return &finiteUniverse{cases: cases, baseline: baseline, totalVectors: total, workspaceSHA: workspaceSHA,
		staticPredicate: staticPredicate, testModels: testModels, testModelDigests: testModelDigests}, nil
}

func validateExecutorEnvironment(environment executor.TaskEnvironment) error {
	if len(environment.Command) == 0 || environment.Timeout <= 0 || environment.Timeout > 24*time.Hour {
		return fmt.Errorf("verifier command or timeout is empty/invalid")
	}
	for _, argument := range environment.Command {
		if argument == "" {
			return fmt.Errorf("verifier command contains an empty argv entry")
		}
	}
	if !environment.ExactEnvironment {
		return fmt.Errorf("verifier must clear ambient state and use an exact frozen environment")
	}
	if strings.TrimSpace(environment.WorkspaceRoot) == "" || strings.TrimSpace(environment.WorkDir) == "" {
		return fmt.Errorf("verifier requires explicit workspace root and command working directory")
	}
	root, err := filepath.Abs(environment.WorkspaceRoot)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve verifier workspace: %w", err)
	}
	workDir, err := filepath.Abs(environment.WorkDir)
	if err != nil {
		return err
	}
	workDir, err = filepath.EvalSymlinks(workDir)
	if err != nil || !within(root, workDir) {
		return fmt.Errorf("verifier working directory is unavailable or outside its workspace")
	}
	info, err := os.Stat(workDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("verifier working directory is not a directory")
	}
	previousName := ""
	for index, entry := range environment.Environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name == "" || strings.Contains(name, "\x00") || (index > 0 && name <= previousName) {
			return fmt.Errorf("verifier environment must be a strictly name-sorted unique KEY=VALUE list")
		}
		previousName = name
	}
	if (environment.PassSignal.ExitCode == nil) == (environment.PassSignal.VerdictFile == nil) {
		return fmt.Errorf("verifier must declare exactly one pass signal")
	}
	if environment.PassSignal.VerdictFile != nil {
		verdict := environment.PassSignal.VerdictFile
		if strings.TrimSpace(verdict.Path) == "" || strings.TrimSpace(verdict.PassValue) == "" {
			return fmt.Errorf("verdict-file pass signal is incomplete")
		}
		path := verdict.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil || !within(root, parent) {
			return fmt.Errorf("verdict-file pass signal escapes the frozen workspace")
		}
	}
	return nil
}

func validateFrozenBinding(executorRoot string, binding ArtifactBinding) error {
	root, err := filepath.Abs(executorRoot)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	workspaceRoot, err := filepath.EvalSymlinks(binding.Frontend.Workspace.Root)
	if err != nil || filepath.Clean(workspaceRoot) != filepath.Clean(root) {
		return fmt.Errorf("frontend and executor workspaces differ")
	}
	if err := semanticir.VerifyArtifact(binding.Frontend.Artifact, binding.Frontend.Source); err != nil {
		return err
	}
	found := false
	for _, entry := range binding.Frontend.Workspace.Entries {
		path := filepath.Join(root, filepath.Clean(entry.Path))
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !within(root, resolved) {
			return fmt.Errorf("workspace entry %q is unavailable or escapes", entry.Path)
		}
		content, err := os.ReadFile(resolved)
		if err != nil {
			return err
		}
		if err := semanticir.VerifyArtifact(entry.Artifact, content); err != nil {
			return err
		}
		if entry.Artifact == binding.Frontend.Artifact {
			found = true
			if !bytes.Equal(content, binding.Frontend.Source) {
				return fmt.Errorf("frontend source differs from frozen workspace bytes")
			}
		}
	}
	if !found {
		return fmt.Errorf("focused artifact is absent from workspace entries")
	}
	treeDigest, err := executor.WorkspaceDigest(root)
	if err != nil || treeDigest != binding.Frontend.Workspace.TreeDigest {
		return fmt.Errorf("frontend workspace tree digest does not match its exact entries")
	}
	return nil
}

func validateExactAssignment(assignment semanticir.Assignment, operation semanticir.Operation, domains map[string]semanticir.Domain) error {
	if len(assignment) != len(operation.DomainIDs) {
		return fmt.Errorf("assignment has %d dimensions, operation requires %d", len(assignment), len(operation.DomainIDs))
	}
	for _, domainID := range operation.DomainIDs {
		valueID, ok := assignment[domainID]
		if !ok {
			return fmt.Errorf("assignment omits domain %q", domainID)
		}
		found := false
		for _, value := range domains[domainID].Values {
			if value.ID == valueID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("assignment value %q is outside domain %q", valueID, domainID)
		}
	}
	return nil
}

func operationAssignments(operation semanticir.Operation, domains map[string]semanticir.Domain) []semanticir.Assignment {
	assignments := []semanticir.Assignment{{}}
	for _, domainID := range operation.DomainIDs {
		var next []semanticir.Assignment
		for _, assignment := range assignments {
			for _, value := range domains[domainID].Values {
				copy := cloneAssignment(assignment)
				copy[domainID] = value.ID
				next = append(next, copy)
			}
		}
		assignments = next
	}
	sort.Slice(assignments, func(i, j int) bool { return assignmentKey(assignments[i]) < assignmentKey(assignments[j]) })
	return assignments
}

func excluded(constraints []semanticir.Constraint, operationID string, assignment semanticir.Assignment) bool {
	for _, constraint := range constraints {
		if constraint.OperationID != operationID || len(constraint.Conditions) != len(assignment) {
			continue
		}
		if equalAssignments(constraint.Conditions, assignment) {
			return true
		}
	}
	return false
}
