package pipeline

import (
	"fmt"
	"sort"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

func lowerEnvironment(configArtifact semanticir.ArtifactRef, manifest taskbundle.Manifest) (*semanticir.EnvironmentModel, error) {
	provenance := semanticir.NewProvenance(configArtifact, semanticir.SourceLocation{
		Path: configArtifact.Path, StartLine: 1, StartColumn: 1,
	}, semanticir.TranslationTranslated)
	configurationDigest, err := semanticir.Digest(manifest.Environment.Configuration)
	if err != nil {
		return nil, fmt.Errorf("digest frozen environment configuration: %w", err)
	}
	model := &semanticir.EnvironmentModel{
		Artifact: configArtifact, Configuration: configArtifact, Identity: manifest.Environment.Identity,
		ConfigDigest: configurationDigest, Provenance: provenance,
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.Kind != string(semanticir.ArtifactEnvironment) {
			continue
		}
		model.SourceArtifacts = append(model.SourceArtifacts, semanticir.ArtifactRef{
			ID: artifact.ID, Kind: semanticir.ArtifactEnvironment, Path: artifact.Path, Digest: artifact.SHA256,
		})
	}
	if len(model.SourceArtifacts) == 0 {
		return nil, fmt.Errorf("frozen manifest declares no environment source artifacts")
	}
	for _, tool := range manifest.Environment.Tools {
		model.Tools = append(model.Tools, toolRef(tool))
	}
	for _, workspace := range manifest.Workspaces {
		exactEnvironment := semanticEnvironment(workspace.Command.Environment)
		environmentDigest, err := semanticir.Digest(exactEnvironment)
		if err != nil {
			return nil, fmt.Errorf("digest exact workspace environment: %w", err)
		}
		state, expectedPass, err := workspaceSemantics(workspace.State)
		if err != nil {
			return nil, err
		}
		signal := semanticir.PassSignal{
			Expected:   workspace.Command.PassSignal.Expected,
			Provenance: provenance,
		}
		switch workspace.Command.PassSignal.Source {
		case taskbundle.SignalExitCode:
			signal.Kind = semanticir.PassSignalExitCode
		case taskbundle.SignalFile:
			signal.Kind = semanticir.PassSignalFile
			signal.Path = workspace.Command.PassSignal.Path
		default:
			return nil, fmt.Errorf("workspace %q pass signal %q cannot be represented exactly", workspace.State, workspace.Command.PassSignal.Source)
		}
		command := semanticir.WorkspaceCommand{
			ID: "command:" + string(workspace.State), WorkspaceID: "workspace:" + string(workspace.State),
			State: state, TreeDigest: workspace.TreeSHA256,
			Command: workspace.Command.Text, WorkingDirectory: workspace.Command.WorkingDirectory,
			Environment: exactEnvironment, EnvironmentDigest: environmentDigest,
			ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: workspace.Command.TimeoutMillis,
			PassSignal: signal, ExpectedPass: expectedPass, ObservedPass: workspace.Result.Passed,
			ExitCode: workspace.Result.ExitCode, StdoutDigest: workspace.Result.StdoutSHA256,
			StderrDigest: workspace.Result.StderrSHA256, SignalValueDigest: workspace.Result.SignalValueSHA256,
			Tools: append([]semanticir.ToolRef(nil), model.Tools...), Provenance: provenance,
		}
		if command.ExpectedPass != command.ObservedPass {
			return nil, fmt.Errorf("workspace %q observation does not match required task semantics", workspace.State)
		}
		model.Commands = append(model.Commands, command)
	}
	constructs := 1 + len(model.SourceArtifacts) + len(model.Tools) + len(model.Commands)
	model.Coverage = semanticir.TranslationCoverage{
		Status: semanticir.TranslationComplete, TotalConstructs: constructs,
		TranslatedConstructs: constructs, Provenance: semanticir.Provenance{
			ArtifactID: provenance.ArtifactID, ArtifactDigest: provenance.ArtifactDigest,
			Location: provenance.Location, Translation: semanticir.TranslationComplete,
		},
	}
	return model, nil
}

func semanticEnvironment(environment map[string]string) []semanticir.EnvironmentVariable {
	keys := make([]string, 0, len(environment))
	for name := range environment {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	variables := make([]semanticir.EnvironmentVariable, 0, len(keys))
	for _, name := range keys {
		variables = append(variables, semanticir.EnvironmentVariable{Name: name, Value: environment[name]})
	}
	return variables
}

func workspaceSemantics(state taskbundle.WorkspaceState) (semanticir.WorkspaceState, bool, error) {
	switch state {
	case taskbundle.BaseOldTests:
		return semanticir.WorkspaceBaseOldTests, true, nil
	case taskbundle.BaseNewTests:
		return semanticir.WorkspaceBaseNewTests, false, nil
	case taskbundle.SolutionNewTests:
		return semanticir.WorkspaceSolutionNewTests, true, nil
	default:
		return "", false, fmt.Errorf("unknown frozen workspace state %q", state)
	}
}
