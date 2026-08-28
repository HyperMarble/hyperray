package semanticir

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

type canonicalExecutionObservation struct {
	OperationID       string             `json:"operation_id"`
	Conditions        Assignment         `json:"conditions"`
	Inputs            map[string]Literal `json:"inputs"`
	OutcomeIDs        []string           `json:"outcome_ids"`
	RawTraceDigest    string             `json:"raw_trace_digest"`
	ExitCode          int                `json:"exit_code"`
	StdoutDigest      string             `json:"stdout_digest"`
	StderrDigest      string             `json:"stderr_digest"`
	SignalValueDigest string             `json:"signal_value_digest"`
}

// ExecutionObservationDigest canonically hashes a complete run independent
// of assignment order and provenance.
func ExecutionObservationDigest(observations []ExecutionObservation) (string, error) {
	values := make([]canonicalExecutionObservation, 0, len(observations))
	for _, observation := range observations {
		outcomeIDs := append([]string(nil), observation.OutcomeIDs...)
		sort.Strings(outcomeIDs)
		rawTraceDigest, _ := Digest(observation.RawOutcome)
		values = append(values, canonicalExecutionObservation{
			observation.Behavior.OperationID, observation.Behavior.Conditions, observation.Inputs, outcomeIDs, rawTraceDigest,
			observation.ExitCode, observation.StdoutDigest, observation.StderrDigest, observation.SignalValueDigest,
		})
	}
	sort.Slice(values, func(i, j int) bool {
		return BehaviorRefKey(BehaviorRef{OperationID: values[i].OperationID, Conditions: values[i].Conditions, Inputs: values[i].Inputs}) < BehaviorRefKey(BehaviorRef{OperationID: values[j].OperationID, Conditions: values[j].Conditions, Inputs: values[j].Inputs})
	})
	return Digest(values)
}

// ExecutionOrderDigest hashes only the declared fresh-process order.
func ExecutionOrderDigest(observations []ExecutionObservation) (string, error) {
	keys := make([]string, 0, len(observations))
	for _, observation := range observations {
		keys = append(keys, BehaviorRefKey(observation.Behavior))
	}
	return Digest(keys)
}

// ExhaustiveExecutionCoreDigest binds the evidence declaration without its
// separately captured central replay transcript.
func ExhaustiveExecutionCoreDigest(evidence ExhaustiveExecutionEvidence) (string, error) {
	copy := evidence
	copy.Replay = ExhaustiveReplayEvidence{}
	return Digest(copy)
}

func validateExhaustiveExecutionEvidence(model ArtifactModel) []Diagnostic {
	if model.Kind != ArtifactCode {
		return nil
	}
	if len(model.CompilerEvidence) > 0 && len(model.ExhaustiveEvidence) > 0 {
		return []Diagnostic{errorDiagnostic(DiagnosticOverlapping, "code artifact mixes compiler-entailment and exhaustive-execution evidence", model.Coverage.Provenance)}
	}
	if len(model.CompilerEvidence) == 0 && len(model.ExhaustiveEvidence) == 0 {
		return []Diagnostic{errorDiagnostic(DiagnosticIncomplete, "code artifact has neither compiler-entailment nor exhaustive-execution evidence", model.Coverage.Provenance)}
	}
	if len(model.ExhaustiveEvidence) > 1 {
		diagnostics := []Diagnostic{errorDiagnostic(DiagnosticOverlapping, "code artifact has multiple exhaustive-execution records", model.Coverage.Provenance)}
		return diagnostics
	}
	var diagnostics []Diagnostic
	for _, evidence := range model.ExhaustiveEvidence {
		diagnostics = append(diagnostics, validateExecutionRecord(model, evidence)...)
	}
	return diagnostics
}

func validateExecutionRecord(model ArtifactModel, evidence ExhaustiveExecutionEvidence) []Diagnostic {
	var diagnostics []Diagnostic
	if evidence.ID == "" || !evidence.Complete || evidence.SourceDigest != model.Artifact.Digest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "exhaustive execution lacks identity/completeness/source binding", evidence.Provenance))
	}
	if validateFactSource(evidence.Provenance, model.Artifact) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidProvenance, "exhaustive execution provenance differs from code artifact", evidence.Provenance))
	}
	if err := validateToolRef(evidence.Tool); err != nil || evidence.Tool != model.Translator {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "exhaustive execution tool differs from translator", evidence.Provenance))
	}
	wantKind := map[Language]CompilerIRKind{LanguagePython: CompilerIRCPythonBytecode, LanguageRust: CompilerIRRustMIR, LanguageCPP: CompilerIRLLVM}[model.Language]
	if evidence.IRKind != wantKind {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution compiler IR kind differs from language", evidence.Provenance))
	}
	for label, digest := range map[string]string{"workspace": evidence.WorkspaceTreeDigest, "emitted IR": evidence.EmittedIRDigest, "executable": evidence.ExecutableDigest} {
		if !ValidDigest(digest) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution has invalid "+label+" digest", evidence.Provenance))
		}
	}
	if len(evidence.Harness) == 0 || evidence.HarnessPath == "" || evidence.HarnessDigest != DigestBytes(evidence.Harness) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution harness bytes/digest mismatch", evidence.Provenance))
	}
	if evidence.WorkingDirectory == "" || len(evidence.Argv) == 0 || evidence.TimeoutMillis <= 0 || !evidence.ClearEnvironment || !evidence.KillProcessGroup {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution invocation/policy is incomplete", evidence.Provenance))
	}
	for _, argument := range evidence.Argv {
		if argument == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution argv has an empty argument", evidence.Provenance))
		}
	}
	if err := validateExactEnvironment(evidence.Environment, evidence.EnvironmentDigest); err != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution environment: "+err.Error(), evidence.Provenance))
	}
	diagnostics = append(diagnostics, ValidateProbeSteps(evidence.Steps, evidence.Provenance)...)
	diagnostics = append(diagnostics, validateExecutionGroundings(model, evidence)...)
	if len(evidence.Runs) < 2 || !ValidDigest(evidence.CompleteAssignmentDigest) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "exhaustive execution needs two full runs and an assignment digest", evidence.Provenance))
	}
	seenRuns, seenTimes, seenOrders := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, run := range evidence.Runs {
		if run.ID == "" {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive execution run ID is empty", run.Provenance))
		}
		if _, exists := seenRuns[run.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "exhaustive execution repeats run "+run.ID, run.Provenance))
		}
		seenRuns[run.ID] = struct{}{}
		started, err := time.Parse(time.RFC3339Nano, run.StartedAtUTC)
		_, offset := started.Zone()
		if err != nil || offset != 0 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "execution run timestamp is not RFC3339 UTC", run.Provenance))
		}
		if _, exists := seenTimes[run.StartedAtUTC]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "execution runs do not have distinct start timestamps", run.Provenance))
		}
		seenTimes[run.StartedAtUTC] = struct{}{}
		observationDigest, observationErr := ExecutionObservationDigest(run.Observations)
		orderDigest, orderErr := ExecutionOrderDigest(run.Observations)
		if observationErr != nil || orderErr != nil || run.ObservationDigest != observationDigest || run.OrderDigest != orderDigest || run.ObservationDigest != evidence.CompleteAssignmentDigest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "execution run digests differ from embedded observations", run.Provenance))
		}
		seenOrders[run.OrderDigest] = struct{}{}
		if run.FreshProcessCount != len(run.Observations) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "execution run did not use one fresh process per assignment", run.Provenance))
		}
		diagnostics = append(diagnostics, validateExecutionObservations(model, run.Observations, run.Provenance)...)
	}
	coreDigest, _ := ExhaustiveExecutionCoreDigest(evidence)
	stepsDigest, _ := Digest(evidence.Steps)
	var generated []ProbeOutput
	for _, step := range evidence.Steps {
		generated = append(generated, step.Outputs...)
	}
	replay := evidence.Replay
	if !replay.Clean || replay.CoreDigest != coreDigest || replay.StepsDigest != stepsDigest || !reflect.DeepEqual(replay.Runs, evidence.Runs) || !reflect.DeepEqual(replay.GeneratedOutputs, generated) || validateFactSource(replay.Provenance, model.Artifact) != nil {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "exhaustive central replay transcript differs from exact evidence/steps/runs/generated outputs", replay.Provenance))
	}
	if len(replay.CleanupSteps) != 0 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive replay cleanup must use central programmatic cleanup paths, not an external command", replay.Provenance))
	}
	previousPath := ""
	for _, path := range replay.CleanupPaths {
		clean := filepath.Clean(path)
		if path == "" || filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || path <= previousPath {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "exhaustive replay cleanup paths are unsafe, unsorted, or duplicated", replay.Provenance))
		}
		previousPath = path
	}
	cleanupDigest, _ := Digest(replay.CleanupPaths)
	if len(replay.CleanupPaths) == 0 || replay.CleanupDigest != cleanupDigest {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "exhaustive replay lacks digest-bound central cleanup paths", replay.Provenance))
	}
	caseCount := len(model.Cases)
	if caseCount > 1 && len(seenOrders) < 2 {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "exhaustive execution did not repeat in an independent assignment order", evidence.Provenance))
	}
	return diagnostics
}

func validateExecutionGroundings(model ArtifactModel, evidence ExhaustiveExecutionEvidence) []Diagnostic {
	var diagnostics []Diagnostic
	operations := map[string]Operation{}
	for _, operation := range model.Operations {
		if operation.Kind != OperationTest {
			operations[operation.ID] = operation
		}
	}
	seen := map[string]struct{}{}
	for _, grounding := range evidence.Groundings {
		operation, exists := operations[grounding.OperationID]
		if !exists || grounding.ID != AssignmentGroundingID(grounding.OperationID, grounding.Conditions) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "exact execution grounding refers outside operation scope or has noncanonical ID", evidence.Provenance))
			continue
		}
		exact, singleton := ExactGroundingInputs(operation, model.Domains, grounding.Conditions)
		if !singleton || !reflect.DeepEqual(exact, grounding.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnsupported, "exhaustive execution grounding is not the unique full input tuple fixed by selected labels", evidence.Provenance))
		}
		key := behaviorKey(grounding.OperationID, grounding.Conditions)
		if _, duplicate := seen[key]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticOverlapping, "exhaustive execution repeats a semantic assignment grounding", evidence.Provenance))
		}
		seen[key] = struct{}{}
	}
	if !sameAssignmentGroundings(evidence.Groundings, model.Groundings) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "exhaustive execution groundings differ from frozen assignment witnesses", evidence.Provenance))
	}
	return diagnostics
}

func validateExecutionObservations(model ArtifactModel, observations []ExecutionObservation, provenance Provenance) []Diagnostic {
	var diagnostics []Diagnostic
	cases := map[string]BehaviorCase{}
	for _, behaviorCase := range model.Cases {
		cases[BehaviorCaseKey(behaviorCase)] = behaviorCase
	}
	seen := map[string]struct{}{}
	steps := map[string]ProbeStep{}
	for _, evidence := range model.ExhaustiveEvidence {
		for _, step := range evidence.Steps {
			steps[step.ID] = step
		}
	}
	for _, observation := range observations {
		key := BehaviorRefKey(observation.Behavior)
		behaviorCase, exists := cases[key]
		if !exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticUnreachable, "execution observes undeclared/constrained behavior "+key, observation.Provenance))
		}
		if _, exists := seen[key]; exists {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticDuplicateID, "execution repeats behavior "+key, observation.Provenance))
		}
		seen[key] = struct{}{}
		raw := observation.RawOutcome
		if err := ValidateExhaustiveRawOutcomeTrace(raw); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "raw execution outcome: "+err.Error(), observation.Provenance))
		}
		canonicalRaw, rawErr := CanonicalJSON(raw)
		if rawErr != nil || !reflect.DeepEqual(canonicalRaw, observation.SignalValue) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "execution signal is not canonical JSON for the raw typed outcome", observation.Provenance))
		}
		expectedID := ""
		operationFound := false
		for _, operation := range model.Operations {
			if operation.ID == observation.Behavior.OperationID {
				operationFound = true
				expectedID, _ = ClassifyRawOutcome(operation, raw, observation.Provenance)
				break
			}
		}
		if !operationFound {
			expectedID = ""
		}
		if len(observation.OutcomeIDs) != 1 || observation.OutcomeIDs[0] != expectedID {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "execution outcome ID was not derived from the raw terminal/effects", observation.Provenance))
		}
		if exists && (len(behaviorCase.OutcomeIDs) != 1 || behaviorCase.OutcomeIDs[0] != expectedID) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "execution outcomes differ from modeled case "+key, observation.Provenance))
		}
		step, stepExists := steps[observation.StepID]
		if !stepExists || step.Kind != ProbeStepRun || step.ExpectedExitCode != observation.ExitCode || step.ExpectedStdoutDigest != observation.StdoutDigest || step.ExpectedStderrDigest != observation.StderrDigest || step.ExpectedSignalDigest != observation.SignalValueDigest {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "execution observation is not bound to its exact direct run step", observation.Provenance))
		}
		if stepExists && (step.SignalExtractor.Kind != ProbeSignalRawOutcomeStdout && step.SignalExtractor.Kind != ProbeSignalRawOutcomeFile) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "exhaustive run step has no typed raw-outcome signal extractor", observation.Provenance))
		}
		if stepExists && step.SignalExtractor.Kind == ProbeSignalRawOutcomeStdout && !reflect.DeepEqual(observation.Stdout, observation.SignalValue) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "stdout signal extractor bytes differ from canonical raw outcome signal", observation.Provenance))
		}
		grounding, grounded := executionGroundingFor(model.ExhaustiveEvidence, observation.Behavior)
		if !grounded || !reflect.DeepEqual(observation.Inputs, grounding.Inputs) || !reflect.DeepEqual(observation.Behavior.Inputs, observation.Inputs) {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidReference, "execution input map differs from the exact semantic assignment grounding", observation.Provenance))
		}
		if observation.StdoutTruncated || observation.StderrTruncated || observation.SignalTruncated || len(observation.Stdout) > 1<<20 || len(observation.Stderr) > 1<<20 || len(observation.SignalValue) > 1<<20 {
			diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, "execution observation output is truncated or exceeds the bounded protocol", observation.Provenance))
		}
		for digestIndex, digest := range []string{observation.StdoutDigest, observation.StderrDigest, observation.SignalValueDigest} {
			if !ValidDigest(digest) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticInvalidInput, "execution observation has invalid output digest", observation.Provenance))
			}
			contents := [][]byte{observation.Stdout, observation.Stderr, observation.SignalValue}[digestIndex]
			if digest != DigestBytes(contents) {
				diagnostics = append(diagnostics, errorDiagnostic(DiagnosticStaleArtifact, "execution observation output bytes/digest mismatch", observation.Provenance))
			}
		}
	}
	if len(seen) != len(cases) {
		diagnostics = append(diagnostics, errorDiagnostic(DiagnosticIncomplete, fmt.Sprintf("execution observes %d cases, want %d", len(seen), len(cases)), provenance))
	}
	return diagnostics
}

func executionGroundingFor(records []ExhaustiveExecutionEvidence, target BehaviorRef) (AssignmentGrounding, bool) {
	var result AssignmentGrounding
	found := false
	for _, record := range records {
		for _, grounding := range record.Groundings {
			if grounding.OperationID == target.OperationID && reflect.DeepEqual(grounding.Conditions, target.Conditions) {
				if found {
					return AssignmentGrounding{}, false
				}
				result = grounding
				found = true
			}
		}
	}
	return result, found
}

func domainValueIDs(domain Domain) map[string]struct{} {
	values := map[string]struct{}{}
	for _, value := range domain.Values {
		values[value.ID] = struct{}{}
	}
	return values
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func executionCasesEqual(left, right []BehaviorCase) bool { return reflect.DeepEqual(left, right) }
