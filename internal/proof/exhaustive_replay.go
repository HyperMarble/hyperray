package proof

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/semanticir"
)

// replayExhaustiveExecution is the central trust boundary for exact-literal
// code models. The frontend transcript is useful only after a fresh,
// isolated replay reproduces its raw typed terminal/effect bytes.
func replayExhaustiveExecution(ctx context.Context, task *semanticir.Task, artifact *semanticir.ArtifactModel, evidence semanticir.ExhaustiveExecutionEvidence) error {
	if task == nil || task.Environment == nil {
		return fmt.Errorf("exhaustive replay has no frozen task environment")
	}
	if !containsToolRef(task.Environment.Tools, evidence.Tool) {
		return fmt.Errorf("exhaustive replay tool %q is absent from the frozen environment", evidence.Tool.Name)
	}
	for _, step := range evidence.Steps {
		if step.Tool != (semanticir.ToolRef{}) && !containsToolRef(task.Environment.Tools, step.Tool) {
			return fmt.Errorf("exhaustive replay step %q tool %q is absent from the frozen environment", step.ID, step.Tool.Name)
		}
		if !reflect.DeepEqual(step.Environment, evidence.Environment) || step.EnvironmentDigest != evidence.EnvironmentDigest {
			return fmt.Errorf("exhaustive replay step %q does not use the evidence's exact frozen environment", step.ID)
		}
	}
	workspaceID := ""
	boundSnapshot := false
	for _, command := range task.Environment.Commands {
		if command.TreeDigest == evidence.WorkspaceTreeDigest && command.EnvironmentDigest == evidence.EnvironmentDigest && reflect.DeepEqual(command.Environment, evidence.Environment) {
			workspaceID = command.WorkspaceID
			boundSnapshot = true
			break
		}
	}
	if !boundSnapshot {
		return fmt.Errorf("exhaustive replay workspace/environment matches no frozen workspace command")
	}
	root, err := exhaustiveWorkspaceRoot(evidence.WorkingDirectory, artifact.Artifact, evidence.WorkspaceTreeDigest)
	if err != nil {
		return err
	}
	plan := executor.ExhaustiveReplayPlan{
		ID: "proof-" + evidence.ID,
		Workspace: executor.ProbeWorkspace{
			ID: workspaceID, Root: root, State: semanticir.WorkspaceSolutionNewTests,
			TreeSHA256: evidence.WorkspaceTreeDigest,
		},
		SourceArtifacts: []semanticir.ArtifactRef{artifact.Artifact},
		Operations:      append([]semanticir.Operation(nil), task.Operations...),
		Evidence:        evidence,
	}
	replayed := executor.ReplayExhaustive(ctx, plan)
	if err := executor.ValidateExhaustiveReplay(replayed); err != nil {
		if len(replayed.Blockers) != 0 {
			return fmt.Errorf("central exhaustive replay blocked: %s: %s", replayed.Blockers[0].Code, replayed.Blockers[0].Detail)
		}
		return fmt.Errorf("central exhaustive replay validation: %w", err)
	}
	semanticReplay, err := executor.SemanticReplay(replayed)
	if err != nil || !reflect.DeepEqual(semanticReplay, evidence.Replay) {
		return fmt.Errorf("central exhaustive replay transcript differs from the embedded semantic replay")
	}
	if err := validateReplayedRawOutcomes(task, artifact, replayed); err != nil {
		return err
	}
	return nil
}

func exhaustiveWorkspaceRoot(workingDirectory string, source semanticir.ArtifactRef, wantedDigest string) (string, error) {
	start, err := filepath.EvalSymlinks(workingDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve exhaustive replay working directory: %w", err)
	}
	info, err := os.Stat(start)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("exhaustive replay working directory is not a readable directory")
	}
	for candidate, depth := filepath.Clean(start), 0; depth < 32; depth++ {
		sourcePath := source.Path
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(candidate, filepath.FromSlash(sourcePath))
		}
		if pathWithinRoot(candidate, sourcePath) {
			if body, readErr := os.ReadFile(sourcePath); readErr == nil && semanticir.DigestBytes(body) == source.Digest {
				if digest, digestErr := executor.WorkspaceDigest(candidate); digestErr == nil && digest == wantedDigest {
					return candidate, nil
				}
			}
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}
	return "", fmt.Errorf("cannot locate the exact frozen workspace root for exhaustive replay")
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, filepath.Clean(path))
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validateReplayedRawOutcomes(task *semanticir.Task, artifact *semanticir.ArtifactModel, replayed executor.ExhaustiveReplayEvidence) error {
	operations := make(map[string]semanticir.Operation, len(task.Operations))
	for _, operation := range task.Operations {
		operations[operation.ID] = operation
	}
	cases := make(map[string]semanticir.BehaviorCase, len(artifact.Cases))
	for _, behaviorCase := range artifact.Cases {
		domains := operations[behaviorCase.OperationID].DomainIDs
		cases[behaviorCase.OperationID+"\x00"+canonicalAssignment(domains, behaviorCase.Conditions)] = behaviorCase
	}
	for _, run := range replayed.Runs {
		for _, observationReplay := range run.Observations {
			actual := observationReplay.Run.SignalValue
			var raw semanticir.RawOutcomeTrace
			decoder := json.NewDecoder(bytes.NewReader(actual))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&raw); err != nil {
				return fmt.Errorf("replayed raw-outcome signal is not closed typed JSON: %w", err)
			}
			canonical, err := semanticir.CanonicalJSON(raw)
			if err != nil || !bytes.Equal(canonical, actual) || semanticir.ValidateExhaustiveRawOutcomeTrace(raw) != nil {
				return fmt.Errorf("replayed process signal is not the canonical raw typed outcome")
			}
			expected := observationReplay.Expected
			operation, exists := operations[expected.Behavior.OperationID]
			if !exists {
				return fmt.Errorf("replayed raw outcome refers outside the modeled operation scope")
			}
			classified, classifyErr := semanticir.ClassifyRawOutcome(operation, raw, expected.Provenance)
			if classifyErr != nil {
				return fmt.Errorf("classify replayed raw terminal/effects: %w", classifyErr)
			}
			key := operation.ID + "\x00" + canonicalAssignment(operation.DomainIDs, expected.Behavior.Conditions)
			behaviorCase, exists := cases[key]
			if !exists || len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OutcomeIDs[0] != classified || len(expected.OutcomeIDs) != 1 || expected.OutcomeIDs[0] != classified {
				return fmt.Errorf("replayed raw terminal/effects classify differently from the frozen behavior case")
			}
		}
	}
	return nil
}
