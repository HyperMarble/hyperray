package pipeline

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/HyperMarble/ray/internal/executor"
	"github.com/HyperMarble/ray/internal/proof"
	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

func executionEnvironment(root string, manifest taskbundle.Manifest) (executor.TaskEnvironment, string, error) {
	for _, workspace := range manifest.Workspaces {
		if workspace.State != taskbundle.SolutionNewTests {
			continue
		}
		workspaceRoot := filepath.Join(root, filepath.FromSlash(workspace.Path))
		workspaceRoot, err := filepath.EvalSymlinks(workspaceRoot)
		if err != nil {
			return executor.TaskEnvironment{}, "", fmt.Errorf("resolve canonical execution workspace: %w", err)
		}
		workDir := filepath.Join(workspaceRoot, filepath.FromSlash(workspace.Command.WorkingDirectory))
		task := executor.TaskEnvironment{
			Command:       []string{workspace.Command.Shell, "-c", workspace.Command.Text},
			WorkspaceRoot: workspaceRoot, WorkspaceSHA256: workspace.TreeSHA256,
			WorkDir: workDir, Timeout: time.Duration(workspace.Command.TimeoutMillis) * time.Millisecond,
			Environment: sortedEnvironment(workspace.Command.Environment), ExactEnvironment: true,
		}
		signal := workspace.Command.PassSignal
		switch signal.Source {
		case taskbundle.SignalExitCode:
			code, err := strconv.Atoi(signal.Expected)
			if err != nil {
				return executor.TaskEnvironment{}, "", fmt.Errorf("invalid frozen exit-code signal: %w", err)
			}
			task.PassSignal = executor.ExitCodeSignal(code)
		case taskbundle.SignalFile:
			verdictPath := filepath.Join(workspaceRoot, filepath.FromSlash(signal.Path))
			task.PassSignal = executor.VerdictFileSignal(verdictPath, signal.Expected)
		default:
			return executor.TaskEnvironment{}, "", fmt.Errorf("frozen pass signal %q cannot be used for executable confirmation", signal.Source)
		}
		return task, workspace.Command.Text, nil
	}
	return executor.TaskEnvironment{}, "", fmt.Errorf("frozen solution+new-tests execution command is absent")
}

func frozenWitnessContext(root string, manifest taskbundle.Manifest, task *semanticir.Task, records []translationRecord, proofResult proof.Result, execution executor.TaskEnvironment) (executor.FrozenWitnessContext, error) {
	if task == nil || task.Environment == nil || task.TestSuite == nil {
		return executor.FrozenWitnessContext{}, fmt.Errorf("witness confirmation requires complete Spec/reference/Test/environment IR")
	}
	referenceDigest, err := semanticir.CanonicalReferenceIRDigest(task)
	if err != nil {
		return executor.FrozenWitnessContext{}, fmt.Errorf("digest reference IR: %w", err)
	}
	testDigest, err := semanticir.CanonicalTestIRDigest(task)
	if err != nil {
		return executor.FrozenWitnessContext{}, fmt.Errorf("digest Test IR: %w", err)
	}
	environmentDigest, err := semanticir.CanonicalEnvironmentIRDigest(task)
	if err != nil {
		return executor.FrozenWitnessContext{}, fmt.Errorf("digest environment IR: %w", err)
	}
	proofDigest, err := semanticir.Digest(proofResult)
	if err != nil {
		return executor.FrozenWitnessContext{}, fmt.Errorf("digest proof result: %w", err)
	}
	workspace, err := solutionManifestWorkspace(manifest)
	if err != nil {
		return executor.FrozenWitnessContext{}, err
	}
	workspaceRoot := filepath.Join(root, filepath.FromSlash(workspace.Path))
	workspaceRoot, err = filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return executor.FrozenWitnessContext{}, fmt.Errorf("resolve witness workspace: %w", err)
	}
	context := executor.FrozenWitnessContext{
		Models: executor.WitnessModelBindings{
			SpecIRSHA256: task.SpecIRDigest, ReferenceIRSHA256: referenceDigest,
			TestIRSHA256: testDigest, EnvironmentIRSHA256: environmentDigest, ProofResultSHA256: proofDigest,
		},
		Workspace: executor.ProbeWorkspace{ID: "workspace:" + string(workspace.State), Root: workspaceRoot, State: semanticir.WorkspaceSolutionNewTests, TreeSHA256: workspace.TreeSHA256},
		Execution: execution,
	}
	for _, record := range records {
		switch record.model.Kind {
		case semanticir.ArtifactCode:
			context.ReferenceArtifacts = appendUniqueArtifact(context.ReferenceArtifacts, record.model.Artifact)
		case semanticir.ArtifactTests:
			context.TestArtifacts = appendUniqueArtifact(context.TestArtifacts, record.model.Artifact)
			if record.request.Configuration != nil {
				context.TestArtifacts = appendUniqueArtifact(context.TestArtifacts, *record.request.Configuration)
			}
		}
	}
	workspaceRef := semanticir.WorkspaceRef{}
	if len(records) != 0 {
		workspaceRef = records[0].request.Workspace
	}
	for _, source := range task.Environment.SourceArtifacts {
		bound, err := bindUniqueWorkspaceArtifactByDigest(&workspaceRef, source)
		if err != nil {
			return executor.FrozenWitnessContext{}, fmt.Errorf("bind environment witness source %q: %w", source.ID, err)
		}
		context.EnvironmentArtifacts = appendUniqueArtifact(context.EnvironmentArtifacts, bound)
	}
	context.ReferenceArtifacts = sortedArtifacts(context.ReferenceArtifacts)
	context.TestArtifacts = sortedArtifacts(context.TestArtifacts)
	context.EnvironmentArtifacts = sortedArtifacts(context.EnvironmentArtifacts)
	context.Tools = append([]semanticir.ToolRef(nil), task.Environment.Tools...)
	sort.Slice(context.Tools, func(i, j int) bool { return context.Tools[i].Name < context.Tools[j].Name })
	if len(context.ReferenceArtifacts) == 0 || len(context.TestArtifacts) == 0 || len(context.EnvironmentArtifacts) == 0 || len(context.Tools) == 0 {
		return executor.FrozenWitnessContext{}, fmt.Errorf("witness context omits independent reference, Test, environment, or tool bindings")
	}
	return context, nil
}

func appendUniqueArtifact(values []semanticir.ArtifactRef, value semanticir.ArtifactRef) []semanticir.ArtifactRef {
	for _, existing := range values {
		if existing.ID == value.ID {
			return values
		}
	}
	return append(values, value)
}

func sortedArtifacts(values []semanticir.ArtifactRef) []semanticir.ArtifactRef {
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values
}
