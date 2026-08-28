package testir

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// SuiteBinding supplies the frozen non-vector evidence that is intentionally
// outside Result: actual test artifacts, optional advisory source models, and
// the typed verifier/environment command selected from the frozen manifest.
type SuiteBinding struct {
	SourceArtifacts   []semanticir.ArtifactRef             `json:"source_artifacts"`
	SourceModels      []semanticir.ArtifactModel           `json:"source_models"`
	Verifier          semanticir.ToolRef                   `json:"verifier"`
	Execution         semanticir.WorkspaceCommand          `json:"execution"`
	RunnerComposition semanticir.RunnerCompositionEvidence `json:"runner_composition"`
	Provenance        semanticir.Provenance                `json:"provenance"`
	Evidence          []semanticir.Provenance              `json:"evidence"`
}

// CompileSuite is the only Result -> semanticir.TestSuiteModel conversion.
// Pipeline and certificate code must use it rather than reproducing digest or
// ordering conventions independently.
func CompileSuite(result Result, binding SuiteBinding) (semanticir.TestSuiteModel, error) {
	if result.Status != StatusComplete {
		return semanticir.TestSuiteModel{}, fmt.Errorf("cannot compile a %s Test IR result", result.Status)
	}
	if err := ValidateEvidence(result); err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	staticSuite, err := CompileStatic(context.Background(), StaticRequest{Task: &result.SemanticTask, TestModels: result.TestModels, Binding: binding})
	if err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	if err := validateSuiteExecution(binding.Execution, binding.Verifier, result); err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	crossVectorDigest, crossAcceptedDigest, err := semanticir.TestVectorDigests(vectorResults(result))
	if err != nil {
		return semanticir.TestSuiteModel{}, err
	}
	staticSuite.CrossCheck = &semanticir.TestCrossCheckEvidence{
		Full: true, Vectors: vectorResults(result), AcceptedVectorCount: result.AcceptedVectors,
		AcceptedVectorsDigest: crossAcceptedDigest, VectorEvidenceDigest: crossVectorDigest,
		Repetitions: result.Repetitions, RunDigests: append([]string(nil), result.RunDigests...), Provenance: binding.Provenance,
	}
	return staticSuite, nil
}

// AcceptedVectorsDigest hashes only canonical accepted semantic vectors, not
// timestamps or command output. Vectors are sorted by their own canonical
// digest, so the value is stable across identical exhaustive runs.
func AcceptedVectorsDigest(result Result) (string, error) {
	_, accepted, err := semanticir.TestVectorDigests(vectorResults(result))
	return accepted, err
}

// VectorEvidenceDigest uses Semantic IR's canonical, provenance-free digest
// of every complete vector plus its Accepted bit. Full executable evidence is
// separately bound by Result.EvidenceSHA256.
func VectorEvidenceDigest(result Result) (string, error) {
	vectors, _, err := semanticir.TestVectorDigests(vectorResults(result))
	return vectors, err
}

func vectorResults(result Result) []semanticir.TestVectorResult {
	vectors := make([]semanticir.TestVectorResult, len(result.Vectors))
	for index, vector := range result.Vectors {
		vectors[index] = semanticir.TestVectorResult{Choices: cloneChoices(vector.Choices), Accepted: vector.TestsPass}
	}
	return vectors
}

func validateSuiteExecution(command semanticir.WorkspaceCommand, verifier semanticir.ToolRef, result Result) error {
	if command.ID == "" || command.WorkspaceID == "" || command.State != semanticir.WorkspaceSolutionNewTests ||
		!semanticir.ValidDigest(command.TreeDigest) || strings.TrimSpace(command.Command) == "" || command.WorkingDirectory == "" ||
		!semanticir.ValidDigest(command.EnvironmentDigest) || command.TimeoutMillis <= 0 || !command.ExpectedPass || !command.ObservedPass {
		return fmt.Errorf("suite execution is not a complete passing solution-new-tests command")
	}
	if !command.ClearEnvironment || !command.KillProcessGroup {
		return fmt.Errorf("suite execution does not declare hermetic environment/process cleanup")
	}
	environmentDigest, err := semanticir.Digest(command.Environment)
	if err != nil || environmentDigest != command.EnvironmentDigest {
		return fmt.Errorf("suite execution exact environment digest differs from its entries")
	}
	rawEnvironment := make([]string, len(command.Environment))
	previousName := ""
	for index, variable := range command.Environment {
		if variable.Name == "" || strings.Contains(variable.Name, "=") || (index > 0 && variable.Name <= previousName) {
			return fmt.Errorf("suite execution environment is not strictly name-sorted and unique")
		}
		previousName = variable.Name
		rawEnvironment[index] = variable.Name + "=" + variable.Value
	}
	commandWorkDir := command.WorkingDirectory
	if !filepath.IsAbs(commandWorkDir) {
		commandWorkDir = filepath.Join(result.Harness.WorkspaceRoot, filepath.FromSlash(commandWorkDir))
	}
	commandWorkDir = filepath.Clean(commandWorkDir)
	if !slices.Equal(rawEnvironment, result.Harness.Environment) || !result.Harness.ExactEnvironment ||
		command.TreeDigest != result.WorkspaceSHA256 || commandWorkDir != filepath.Clean(result.Harness.WorkDir) ||
		command.TimeoutMillis != result.Harness.Timeout.Milliseconds() {
		return fmt.Errorf("suite execution differs from the exhaustive frozen harness/workspace")
	}
	if len(result.Harness.Command) != 3 || result.Harness.Command[1] != "-c" || result.Harness.Command[2] != command.Command {
		return fmt.Errorf("suite execution command text differs from the exhaustive argv")
	}
	if result.Harness.PassSignal.ExitCode != nil {
		if command.PassSignal.Kind != semanticir.PassSignalExitCode || command.PassSignal.Expected != fmt.Sprint(*result.Harness.PassSignal.ExitCode) {
			return fmt.Errorf("suite execution exit-code signal differs from the exhaustive harness")
		}
	} else if result.Harness.PassSignal.VerdictFile != nil {
		verdict := result.Harness.PassSignal.VerdictFile
		commandPath := command.PassSignal.Path
		if !filepath.IsAbs(commandPath) {
			commandPath = filepath.Join(result.Harness.WorkspaceRoot, filepath.FromSlash(commandPath))
		}
		if command.PassSignal.Kind != semanticir.PassSignalFile || filepath.Clean(commandPath) != filepath.Clean(verdict.Path) || command.PassSignal.Expected != verdict.PassValue {
			return fmt.Errorf("suite execution verdict-file signal differs from the exhaustive harness")
		}
	} else {
		return fmt.Errorf("exhaustive harness has no pass signal")
	}
	if command.ExitCode != *result.Execution.Baseline.ExitCode || command.StdoutDigest != result.Execution.Baseline.StdoutSHA256 ||
		command.StderrDigest != result.Execution.Baseline.StderrSHA256 || command.SignalValueDigest != result.Execution.Baseline.SignalValueSHA256 {
		return fmt.Errorf("suite execution result differs from exhaustive executor baseline")
	}
	foundVerifier := false
	for _, tool := range command.Tools {
		if reflect.DeepEqual(tool, verifier) {
			foundVerifier = true
		}
	}
	if !foundVerifier {
		return fmt.Errorf("suite verifier tool is absent from the frozen workspace command")
	}
	return nil
}

func containsProvenance(values []semanticir.Provenance, wanted semanticir.Provenance) bool {
	for _, value := range values {
		if reflect.DeepEqual(value, wanted) {
			return true
		}
	}
	return false
}
