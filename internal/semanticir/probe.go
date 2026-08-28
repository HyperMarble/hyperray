package semanticir

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateProbeSteps validates an ordered, shell-free process transcript.
// Each step is replayed by executing Tool.Path directly with the exact argv,
// stdin, environment, working directory, timeout and process-group policy.
func ValidateProbeSteps(steps []ProbeStep, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	if len(steps) == 0 {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "direct probe has no steps", provenance)}
	}
	seen := map[string]struct{}{}
	generated := map[string]ProbeOutput{}
	runSeen := false
	cleanupSeen := false
	for index, step := range steps {
		if step.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe step ID is empty", step.Provenance))
		} else if _, exists := seen[step.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "probe repeats step "+step.ID, step.Provenance))
		}
		seen[step.ID] = struct{}{}
		if step.Kind != ProbeStepSetup && step.Kind != ProbeStepRun && step.Kind != ProbeStepCleanup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, fmt.Sprintf("probe step %q has invalid kind", step.ID), step.Provenance))
		}
		if step.Kind == ProbeStepSetup && runSeen {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe setup step follows a run step", step.Provenance))
		}
		if step.Kind == ProbeStepRun && cleanupSeen {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe run step follows cleanup", step.Provenance))
		}
		runSeen = runSeen || step.Kind == ProbeStepRun
		cleanupSeen = cleanupSeen || step.Kind == ProbeStepCleanup
		hasTool := step.Tool != (ToolRef{})
		hasGenerated := step.GeneratedExecutableID != ""
		if hasTool == hasGenerated || (hasGenerated && step.Kind != ProbeStepRun) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe step must use exactly one frozen tool or prior generated executable", step.Provenance))
		}
		if hasTool {
			if err := validateToolRef(step.Tool); err != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe tool: "+err.Error(), step.Provenance))
			}
		} else if output, exists := generated[step.GeneratedExecutableID]; !exists || !output.Executable {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "probe run does not reference an earlier executable output", step.Provenance))
		}
		name := strings.ToLower(filepath.Base(step.Tool.Path))
		if name == "sh" || name == "bash" || name == "zsh" || name == "dash" || name == "cmd.exe" || name == "powershell" || name == "pwsh" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe steps may not invoke a shell", step.Provenance))
		}
		if step.WorkingDirectory == "" || step.TimeoutMillis <= 0 || !step.ClearEnvironment || !step.KillProcessGroup {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "probe step lacks workdir/timeout/hermetic process policy", step.Provenance))
		}
		for _, argument := range step.Argv {
			if argument == "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe argv contains an empty argument", step.Provenance))
			}
		}
		if step.StdinDigest != DigestBytes(step.Stdin) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "probe stdin bytes/digest mismatch", step.Provenance))
		}
		if err := validateExactEnvironment(step.Environment, step.EnvironmentDigest); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe environment: "+err.Error(), step.Provenance))
		}
		for _, digest := range []string{step.ExpectedStdoutDigest, step.ExpectedStderrDigest, step.ExpectedSignalDigest} {
			if !ValidDigest(digest) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe expected output digest is invalid", step.Provenance))
			}
		}
		switch step.SignalExtractor.Kind {
		case ProbeSignalNone:
			if step.SignalExtractor.Path != "" {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe none signal extractor has a path", step.Provenance))
			}
		case ProbeSignalRawOutcomeStdout:
			if step.Kind != ProbeStepRun || step.SignalExtractor.Path != "" || step.ExpectedSignalDigest != step.ExpectedStdoutDigest {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "stdout raw-outcome extractor is not bound to run stdout", step.Provenance))
			}
		case ProbeSignalRawOutcomeFile:
			clean := filepath.Clean(step.SignalExtractor.Path)
			if step.Kind != ProbeStepRun || step.SignalExtractor.Path == "" || filepath.IsAbs(step.SignalExtractor.Path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "file raw-outcome extractor path is unsafe", step.Provenance))
			}
		default:
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe has unsupported signal extractor", step.Provenance))
		}
		if validateProvenance(step.Provenance) != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "probe step has invalid provenance", step.Provenance))
		}
		for _, output := range step.Outputs {
			clean := filepath.Clean(output.Path)
			if step.Kind != ProbeStepSetup || output.ID == "" || output.Path == "" || filepath.IsAbs(output.Path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || !ValidDigest(output.AfterDigest) || (output.ExistedBefore && !ValidDigest(output.BeforeDigest)) || (!output.ExistedBefore && output.BeforeDigest != "") || validateProvenance(output.Provenance) != nil {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "probe output transition is invalid or escapes its working directory", step.Provenance))
			}
			if _, exists := generated[output.ID]; exists {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "probe repeats generated output "+output.ID, step.Provenance))
			}
			generated[output.ID] = output
		}
		_ = index
	}
	if !runSeen {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "direct probe has no run step", provenance))
	}
	return diagnostics
}
