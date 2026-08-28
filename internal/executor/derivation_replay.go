package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// DerivationReplayPlan binds one compiler semantic graph to the frozen source
// and solution workspace from which its IR derivation must be reproduced.
// The executor does not decode or invent graph semantics; it only confirms
// the graph's exact frontend-declared derivation transcript and IR bytes.
type DerivationReplayPlan struct {
	ID              string                           `json:"id"`
	Workspace       ProbeWorkspace                   `json:"workspace"`
	SourceArtifacts []semanticir.ArtifactRef         `json:"source_artifacts"`
	Graph           semanticir.CompilerSemanticGraph `json:"graph"`
}

// DerivationReplayRun is one complete derivation in a new disposable copy.
// Two such runs are mandatory so process-local or workspace-local state
// cannot silently make a nondeterministic compiler transcript authoritative.
type DerivationReplayRun struct {
	ID                    string                              `json:"id"`
	DerivationSteps       []ExhaustiveReplayCommandEvidence   `json:"derivation_steps"`
	DecoderSteps          []ExhaustiveReplayCommandEvidence   `json:"decoder_steps"`
	GeneratedOutputs      []ExhaustiveGeneratedOutputEvidence `json:"generated_outputs"`
	IRStepID              string                              `json:"ir_step_id"`
	IR                    []byte                              `json:"ir"`
	IRSHA256              string                              `json:"ir_sha256"`
	DecoderStepID         string                              `json:"decoder_step_id"`
	DecoderOutput         []byte                              `json:"decoder_output"`
	DecoderOutputSHA256   string                              `json:"decoder_output_sha256"`
	Isolation             IsolationEvidence                   `json:"isolation"`
	ToolsVerifiedAfterRun bool                                `json:"tools_verified_after_run"`
	Complete              bool                                `json:"complete"`
	Error                 string                              `json:"error,omitempty"`
}

// DerivationReplayEvidence is the self-contained certificate-facing result
// of two exact, isolated compiler-IR derivations.
type DerivationReplayEvidence struct {
	Plan                    DerivationReplayPlan  `json:"plan"`
	PlanSHA256              string                `json:"plan_sha256"`
	GraphSHA256             string                `json:"graph_sha256"`
	WorkspaceSHA256         string                `json:"workspace_sha256"`
	SourceBindings          []BindingEvidence     `json:"source_bindings"`
	ToolBindings            []BindingEvidence     `json:"tool_bindings"`
	Runs                    []DerivationReplayRun `json:"runs"`
	Deterministic           bool                  `json:"deterministic"`
	Status                  Status                `json:"status"`
	OriginalBeforeSHA256    string                `json:"original_before_sha256"`
	OriginalAfterSHA256     string                `json:"original_after_sha256"`
	OriginalWorkspaceIntact bool                  `json:"original_workspace_intact"`
	Blockers                []Blocker             `json:"blockers,omitempty"`
}

type preparedDerivationReplay struct {
	plan          DerivationReplayPlan
	root          string
	sources       []preparedBinding
	tools         []preparedBinding
	derive        []semanticir.ProbeStep
	decode        []semanticir.ProbeStep
	outputs       map[string]preparedReplayOutput
	irStepID      string
	decoderStepID string
}

// ReplayDerivation replays a CompilerSemanticGraph derivation twice. Every
// run starts from a byte-identical workspace copy, uses an exact cleared
// environment and frozen tools, and must reproduce Graph.IR byte-for-byte.
func ReplayDerivation(ctx context.Context, plan DerivationReplayPlan) (result DerivationReplayEvidence) {
	result.Plan = plan
	result.Status = StatusProofBlocked
	result.PlanSHA256, _ = semanticir.Digest(plan)
	result.GraphSHA256, _ = semanticir.Digest(plan.Graph)
	result.WorkspaceSHA256 = plan.Workspace.TreeSHA256
	if ctx == nil {
		result.Blockers = append(result.Blockers, derivationBlocker(plan.ID, "nil-context", "execution context is nil"))
		return result
	}
	prepared, blocker := prepareDerivationReplay(plan)
	if blocker != nil {
		result.Blockers = append(result.Blockers, *blocker)
		return result
	}
	result.OriginalBeforeSHA256 = plan.Workspace.TreeSHA256
	for _, source := range prepared.sources {
		result.SourceBindings = append(result.SourceBindings, BindingEvidence{
			ID: source.ref.ID, Path: source.path, ExpectedSHA256: source.ref.Digest,
			ObservedSHA256: source.digest, Verified: source.ref.Digest == source.digest,
		})
	}
	for _, tool := range prepared.tools {
		result.ToolBindings = append(result.ToolBindings, BindingEvidence{
			ID: tool.tool.Name, Path: tool.tool.Path, ExpectedSHA256: tool.tool.Digest,
			ObservedSHA256: tool.digest, Version: tool.version, Verified: tool.tool.Digest == tool.digest,
		})
	}
	defer func() {
		after, err := WorkspaceDigest(prepared.root)
		if err != nil {
			result.Blockers = append(result.Blockers, derivationBlocker(plan.ID, "original-workspace-unreadable", err.Error()))
		} else {
			result.OriginalAfterSHA256 = after
			result.OriginalWorkspaceIntact = after == plan.Workspace.TreeSHA256
			if !result.OriginalWorkspaceIntact {
				result.Blockers = append(result.Blockers, derivationBlocker(plan.ID, "original-workspace-mutated", "frozen workspace changed during derivation replay"))
			}
		}
		if err := verifyPreparedProbeTools(prepared.tools); err != nil {
			result.Blockers = append(result.Blockers, derivationBlocker(plan.ID, "derivation-tool-mutated", err.Error()))
		}
		if len(result.Runs) == 2 && result.Runs[0].Complete && result.Runs[1].Complete {
			result.Deterministic = bytes.Equal(result.Runs[0].IR, result.Runs[1].IR) && bytes.Equal(result.Runs[0].IR, plan.Graph.IR) &&
				bytes.Equal(result.Runs[0].DecoderOutput, result.Runs[1].DecoderOutput) && bytes.Equal(result.Runs[0].DecoderOutput, plan.Graph.DecoderOutput)
		}
		if len(result.Blockers) == 0 && result.OriginalWorkspaceIntact && result.Deterministic {
			result.Status = StatusConfirmed
		}
	}()

	for index := 0; index < 2; index++ {
		run := replayDerivationOnce(ctx, prepared, fmt.Sprintf("repetition-%d", index+1))
		result.Runs = append(result.Runs, run)
		if run.Error != "" {
			result.Blockers = append(result.Blockers, derivationBlocker(plan.ID, "derivation-replay-mismatch", fmt.Sprintf("%s: %s", run.ID, run.Error)))
			return result
		}
	}
	return result
}

func derivationBlocker(planID, code, detail string) Blocker {
	return Blocker{Stage: "derivation-replay", PlanID: planID, Code: code, Detail: detail}
}

func prepareDerivationReplay(plan DerivationReplayPlan) (preparedDerivationReplay, *Blocker) {
	block := func(code, detail string) (preparedDerivationReplay, *Blocker) {
		value := derivationBlocker(plan.ID, code, detail)
		return preparedDerivationReplay{}, &value
	}
	if strings.TrimSpace(plan.ID) == "" {
		return block("invalid-derivation-plan", "derivation replay plan ID is empty")
	}
	graph := plan.Graph
	if len(graph.IR) == 0 || graph.IRDigest != digestBytes(graph.IR) || !validDigest(graph.IRDigest) || len(graph.DecoderOutput) == 0 || graph.DecoderOutputDigest != digestBytes(graph.DecoderOutput) || !validDigest(graph.DecoderOutputDigest) || graph.SourceDigest == "" || !validDigest(graph.SourceDigest) {
		return block("invalid-derivation-graph", "graph IR/decoder/source bytes and digests are incomplete or inconsistent")
	}
	if err := validateDerivationDecoder(graph); err != nil {
		return block("invalid-derivation-decoder", err.Error())
	}
	if plan.Workspace.ID == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || !filepath.IsAbs(plan.Workspace.Root) || filepath.Clean(plan.Workspace.Root) != plan.Workspace.Root || !validDigest(plan.Workspace.TreeSHA256) || graph.WorkspaceTreeDigest != plan.Workspace.TreeSHA256 {
		return block("invalid-derivation-workspace", "graph must bind the canonical frozen solution+new-tests workspace")
	}
	root, err := filepath.EvalSymlinks(plan.Workspace.Root)
	if err != nil {
		return block("invalid-derivation-workspace", err.Error())
	}
	root = filepath.Clean(root)
	if digest, digestErr := WorkspaceDigest(root); digestErr != nil || digest != plan.Workspace.TreeSHA256 {
		return block("stale-derivation-workspace", fmt.Sprintf("workspace digest differs from graph binding: observed=%s expected=%s error=%v", digest, plan.Workspace.TreeSHA256, digestErr))
	}
	if len(plan.SourceArtifacts) == 0 {
		return block("missing-derivation-sources", "derivation replay declares no frozen source artifacts")
	}
	sources := make([]preparedBinding, 0, len(plan.SourceArtifacts))
	seenSourceIDs, seenSourcePaths, sourceDigestBound := map[string]bool{}, map[string]bool{}, false
	for _, source := range plan.SourceArtifacts {
		if source.ID == "" || source.Path == "" || !validDigest(source.Digest) {
			return block("invalid-derivation-source", "source binding is incomplete")
		}
		path, pathErr := resolveProbeExisting(root, source.Path, false)
		if pathErr != nil {
			return block("derivation-source-path", pathErr.Error())
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil || digestBytes(body) != source.Digest {
			return block("stale-derivation-source", fmt.Sprintf("source %q differs from its frozen digest", source.ID))
		}
		if seenSourceIDs[source.ID] || seenSourcePaths[path] {
			return block("duplicate-derivation-source", "source IDs and resolved paths must be unique")
		}
		seenSourceIDs[source.ID], seenSourcePaths[path] = true, true
		sourceDigestBound = sourceDigestBound || source.Digest == graph.SourceDigest
		sources = append(sources, preparedBinding{ref: source, path: path, digest: source.Digest})
	}
	if !sourceDigestBound || graph.Provenance.ArtifactDigest != graph.SourceDigest || !seenSourceIDs[graph.Provenance.ArtifactID] {
		return block("unbound-derivation-source", "graph source/provenance digest is not bound to a frozen artifact")
	}
	if err := validateDerivationEnvironment(graph.Environment, graph.EnvironmentDigest); err != nil {
		return block("invalid-derivation-environment", err.Error())
	}
	if diagnostics := semanticir.ValidateProbeSteps(graph.DerivationSteps, graph.Provenance); semanticir.HasErrors(diagnostics) {
		return block("invalid-derivation-steps", diagnostics[0].Message)
	}
	if diagnostics := semanticir.ValidateProbeSteps(graph.DecoderSteps, graph.Provenance); semanticir.HasErrors(diagnostics) {
		return block("invalid-decoder-steps", diagnostics[0].Message)
	}
	if len(graph.DerivationSteps) == 0 || len(graph.DecoderSteps) == 0 {
		return block("invalid-derivation-steps", "graph has no derivation or decoder steps")
	}

	toolsByKey := map[string]preparedBinding{}
	bindTool := func(tool semanticir.ToolRef) error {
		if tool == (semanticir.ToolRef{}) {
			return nil
		}
		if tool.Name == "" || tool.Version == "" || !filepath.IsAbs(tool.Path) || !validDigest(tool.Digest) {
			return fmt.Errorf("tool %q has incomplete frozen identity", tool.Name)
		}
		resolved, resolveErr := filepath.EvalSymlinks(tool.Path)
		if resolveErr != nil {
			return fmt.Errorf("resolve tool %q: %w", tool.Name, resolveErr)
		}
		resolved = filepath.Clean(resolved)
		info, statErr := os.Lstat(resolved)
		body, readErr := os.ReadFile(resolved)
		if statErr != nil || readErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || digestBytes(body) != tool.Digest {
			return fmt.Errorf("tool %q is not the declared regular executable", tool.Name)
		}
		key := tool.Name + "\x00" + resolved
		if prior, exists := toolsByKey[key]; exists && !reflect.DeepEqual(prior.tool, tool) {
			return fmt.Errorf("tool %q has inconsistent frozen bindings", tool.Name)
		}
		toolsByKey[key] = preparedBinding{tool: tool, path: resolved, digest: tool.Digest, version: tool.Version}
		return nil
	}
	if err := bindTool(graph.Tool); err != nil {
		return block("invalid-derivation-tool", err.Error())
	}
	primaryToolUsed := false
	outputs := map[string]preparedReplayOutput{}
	outputPaths := map[string]bool{}
	irStepID := ""
	decoderStepID := ""
	seenSteps := map[string]bool{}
	type phase struct {
		name   string
		steps  []semanticir.ProbeStep
		target string
		bound  *string
	}
	for _, transcript := range []phase{
		{name: "derivation", steps: graph.DerivationSteps, target: graph.IRDigest, bound: &irStepID},
		{name: "decoder", steps: graph.DecoderSteps, target: graph.DecoderOutputDigest, bound: &decoderStepID},
	} {
		for _, step := range transcript.steps {
			if seenSteps[step.ID] {
				return block("invalid-derivation-steps", "derivation and decoder step IDs must be globally unique")
			}
			seenSteps[step.ID] = true
			if step.Kind == semanticir.ProbeStepCleanup {
				return block("invalid-derivation-cleanup", "derivation cleanup is executor-owned and may not invoke another process")
			}
			if !reflect.DeepEqual(step.Environment, graph.Environment) || step.EnvironmentDigest != graph.EnvironmentDigest {
				return block("cross-derivation-environment", fmt.Sprintf("step %q environment differs from the graph environment", step.ID))
			}
			if step.SignalExtractor.Kind != semanticir.ProbeSignalNone || step.SignalExtractor.Path != "" || step.ExpectedSignalDigest != digestBytes(nil) {
				return block("invalid-derivation-signal", "compiler derivation steps may expose only exact process stdout/stderr, not semantic signals")
			}
			if err := validateReplayStepRoot(root, step); err != nil {
				return block("invalid-derivation-steps", err.Error())
			}
			if step.Tool != (semanticir.ToolRef{}) {
				if err := bindTool(step.Tool); err != nil {
					return block("invalid-derivation-tool", err.Error())
				}
				primaryToolUsed = primaryToolUsed || reflect.DeepEqual(step.Tool, graph.Tool)
			}
			if step.Kind == semanticir.ProbeStepSetup {
				workRel, _ := replayWorkRelative(root, step.WorkingDirectory)
				for _, output := range step.Outputs {
					if output.ExistedBefore || output.BeforeDigest != "" {
						return block("nonfresh-derivation-output", fmt.Sprintf("setup output %q must be fresh", output.ID))
					}
					pathRel, pathErr := replayOutputRelative(root, workRel, output.Path)
					if pathErr != nil {
						return block("invalid-derivation-output-path", pathErr.Error())
					}
					if _, duplicate := outputs[output.ID]; duplicate {
						return block("duplicate-derivation-output", fmt.Sprintf("output ID %q is duplicated", output.ID))
					}
					if outputPaths[pathRel] {
						return block("duplicate-derivation-output", fmt.Sprintf("output %q aliases another generated path", output.ID))
					}
					for _, source := range sources {
						if samePath(source.path, filepath.Join(root, pathRel)) {
							return block("derivation-output-source-collision", fmt.Sprintf("output %q aliases frozen source", output.ID))
						}
					}
					outputs[output.ID] = preparedReplayOutput{output: output, producerID: step.ID, workRel: workRel, pathRel: pathRel}
					outputPaths[pathRel] = true
				}
			}
			if step.Kind == semanticir.ProbeStepRun && step.ExpectedStdoutDigest == transcript.target {
				if *transcript.bound != "" {
					return block("ambiguous-derivation-output", fmt.Sprintf("multiple %s run steps claim the retained output digest", transcript.name))
				}
				*transcript.bound = step.ID
			}
		}
	}
	if !primaryToolUsed || irStepID == "" || decoderStepID == "" {
		return block("unbound-derivation-tool", "transcripts do not bind the primary graph tool and exact IR/decoder-producing runs")
	}
	tools := make([]preparedBinding, 0, len(toolsByKey))
	for _, tool := range toolsByKey {
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].tool.Name+"\x00"+tools[i].tool.Path < tools[j].tool.Name+"\x00"+tools[j].tool.Path
	})
	return preparedDerivationReplay{
		plan: plan, root: root, sources: sources, tools: tools,
		derive:  append([]semanticir.ProbeStep(nil), graph.DerivationSteps...),
		decode:  append([]semanticir.ProbeStep(nil), graph.DecoderSteps...),
		outputs: outputs, irStepID: irStepID, decoderStepID: decoderStepID,
	}, nil
}

func validateDerivationEnvironment(environment []semanticir.EnvironmentVariable, digest string) error {
	previous := ""
	for index, variable := range environment {
		if variable.Name == "" || strings.ContainsAny(variable.Name, "=\x00") || strings.ContainsRune(variable.Value, '\x00') || !utf8.ValidString(variable.Name) || !utf8.ValidString(variable.Value) || index > 0 && variable.Name <= previous {
			return fmt.Errorf("environment variables are not canonical, strictly sorted, and unique")
		}
		previous = variable.Name
	}
	observed, err := semanticir.Digest(environment)
	if err != nil || observed != digest {
		return fmt.Errorf("environment bytes differ from their exact digest")
	}
	return nil
}

func validateDerivationDecoder(graph semanticir.CompilerSemanticGraph) error {
	decoder := json.NewDecoder(bytes.NewReader(graph.DecoderOutput))
	decoder.DisallowUnknownFields()
	var decoded semanticir.CompilerDecodedSemantics
	if err := decoder.Decode(&decoded); err != nil {
		return fmt.Errorf("decode compiler typed semantic graph: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("compiler typed semantic graph contains trailing JSON")
	}
	canonical, err := semanticir.CanonicalCompilerDecoderOutput(&graph)
	if err != nil || !bytes.Equal(canonical, graph.DecoderOutput) || !reflect.DeepEqual(decoded, graph.DecodedSemantics()) {
		return fmt.Errorf("decoder output is not the strict canonical complete typed semantic graph")
	}
	return nil
}

func replayDerivationOnce(ctx context.Context, prepared preparedDerivationReplay, runID string) (record DerivationReplayRun) {
	record.ID, record.IRStepID, record.DecoderStepID = runID, prepared.irStepID, prepared.decoderStepID
	tempParent, runRoot, copiedDigest, err := makeProbeWorkspaceCopy(prepared.root, prepared.plan.Workspace.TreeSHA256)
	record.Isolation = IsolationEvidence{
		OriginalRoot: prepared.root, ExpectedSHA256: prepared.plan.Workspace.TreeSHA256,
		OriginalBeforeSHA256: prepared.plan.Workspace.TreeSHA256, CopyBeforeSHA256: copiedDigest,
	}
	if err != nil {
		record.Error = "create fresh derivation workspace: " + err.Error()
		return record
	}
	record.Isolation.IsolatedRoot = runRoot
	generated := map[string]*ExhaustiveGeneratedOutputEvidence{}
	defer func() {
		ids := make([]string, 0, len(generated))
		for id := range generated {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			output := generated[id]
			path := filepath.Join(runRoot, filepath.FromSlash(output.Path))
			removeErr := os.Remove(path)
			if removeErr != nil && !os.IsNotExist(removeErr) {
				output.RemovalError = removeErr.Error()
				record.Error = appendError(record.Error, fmt.Sprintf("remove generated output %q: %v", id, removeErr))
			} else {
				_, statErr := os.Lstat(path)
				output.RemovedAfterRun = os.IsNotExist(statErr)
			}
			record.GeneratedOutputs = append(record.GeneratedOutputs, *output)
		}
		originalTask := TaskEnvironment{WorkspaceRoot: prepared.root, WorkspaceSHA256: prepared.plan.Workspace.TreeSHA256}
		if cleanupErr := finalizeIsolation(tempParent, runRoot, originalTask, &record.Isolation); cleanupErr != nil {
			record.Error = appendError(record.Error, cleanupErr.Error())
		}
		if !record.Isolation.IsolatedRemoved || !record.Isolation.OriginalIntact {
			record.Error = appendError(record.Error, "derivation workspace cleanup or original-workspace preservation failed")
		}
		record.Complete = record.Error == "" && record.ToolsVerifiedAfterRun &&
			bytes.Equal(record.IR, prepared.plan.Graph.IR) && record.IRSHA256 == prepared.plan.Graph.IRDigest &&
			bytes.Equal(record.DecoderOutput, prepared.plan.Graph.DecoderOutput) && record.DecoderOutputSHA256 == prepared.plan.Graph.DecoderOutputDigest &&
			len(record.DerivationSteps) == len(prepared.derive) && len(record.DecoderSteps) == len(prepared.decode)
	}()

	replayPrepared := preparedExhaustiveReplay{root: prepared.root, tools: prepared.tools, outputs: prepared.outputs}
	type replayPhase struct {
		steps    []semanticir.ProbeStep
		commands *[]ExhaustiveReplayCommandEvidence
		targetID string
		expected []byte
		output   *[]byte
		digest   *string
	}
	for _, transcript := range []replayPhase{
		{steps: prepared.derive, commands: &record.DerivationSteps, targetID: prepared.irStepID, expected: prepared.plan.Graph.IR, output: &record.IR, digest: &record.IRSHA256},
		{steps: prepared.decode, commands: &record.DecoderSteps, targetID: prepared.decoderStepID, expected: prepared.plan.Graph.DecoderOutput, output: &record.DecoderOutput, digest: &record.DecoderOutputSHA256},
	} {
		for _, step := range transcript.steps {
			if step.Kind == semanticir.ProbeStepSetup {
				for _, output := range step.Outputs {
					declared := prepared.outputs[output.ID]
					path := filepath.Join(runRoot, filepath.FromSlash(declared.pathRel))
					if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
						record.Error = fmt.Sprintf("create output parent for %q: %v", output.ID, err)
						return record
					}
					if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
						record.Error = fmt.Sprintf("generated output %q was not fresh", output.ID)
						return record
					}
				}
			}
			generatedPath := ""
			if step.GeneratedExecutableID != "" {
				output := generated[step.GeneratedExecutableID]
				if output == nil || !output.Output.Executable {
					record.Error = fmt.Sprintf("step %q has no fresh executable output %q", step.ID, step.GeneratedExecutableID)
					return record
				}
				if err := verifyReplayOutput(runRoot, output, true); err != nil {
					record.Error = err.Error()
					return record
				}
				generatedPath = filepath.Join(runRoot, filepath.FromSlash(output.Path))
			}
			command := runExhaustiveReplayStep(ctx, replayPrepared, runRoot, step, generatedPath)
			*transcript.commands = append(*transcript.commands, command)
			if step.GeneratedExecutableID != "" {
				if err := verifyReplayOutput(runRoot, generated[step.GeneratedExecutableID], false); err != nil {
					record.Error = err.Error()
					return record
				}
			}
			if !command.Passed {
				record.Error = fmt.Sprintf("step %q did not reproduce its exact process record: %s", step.ID, command.Error)
				return record
			}
			if step.Kind == semanticir.ProbeStepSetup {
				for _, output := range step.Outputs {
					declared := prepared.outputs[output.ID]
					inspected, inspectErr := inspectReplayOutput(filepath.Join(runRoot, filepath.FromSlash(declared.pathRel)), declared.pathRel, output, step.ID)
					if inspectErr != nil {
						record.Error = inspectErr.Error()
						return record
					}
					generated[output.ID] = &inspected
				}
			}
			if step.ID == transcript.targetID {
				*transcript.output = append([]byte(nil), command.Stdout...)
				*transcript.digest = digestBytes(*transcript.output)
				if !bytes.Equal(*transcript.output, transcript.expected) || *transcript.digest != step.ExpectedStdoutDigest {
					record.Error = "actual compiler transcript bytes differ from the retained semantic graph"
					return record
				}
			}
		}
	}
	for _, output := range generated {
		if !output.VerifiedAfterRun {
			if err := verifyReplayOutput(runRoot, output, false); err != nil {
				record.Error = err.Error()
				return record
			}
		}
	}
	record.ToolsVerifiedAfterRun = verifyPreparedProbeTools(prepared.tools) == nil
	if !record.ToolsVerifiedAfterRun {
		record.Error = "frozen derivation tool binding changed during replay"
	}
	return record
}

// ValidateDerivationReplay checks a confirmed replay without consulting the
// mutable workspace. Issuers must additionally cross-bind Plan.Graph and the
// source/tool/workspace records to the authoritative model and manifest.
func ValidateDerivationReplay(evidence DerivationReplayEvidence) error {
	if evidence.Status != StatusConfirmed || len(evidence.Blockers) != 0 || !evidence.Deterministic || !evidence.OriginalWorkspaceIntact || len(evidence.Runs) != 2 {
		return fmt.Errorf("derivation replay is not a complete deterministic confirmation")
	}
	plan := evidence.Plan
	if evidence.PlanSHA256 != mustProbeDigest(plan) || evidence.GraphSHA256 != mustProbeDigest(plan.Graph) || evidence.WorkspaceSHA256 != plan.Workspace.TreeSHA256 || evidence.OriginalBeforeSHA256 != plan.Workspace.TreeSHA256 || evidence.OriginalAfterSHA256 != plan.Workspace.TreeSHA256 {
		return fmt.Errorf("derivation replay plan, graph, or workspace digests are inconsistent")
	}
	if err := validateDerivationPlanRecord(plan); err != nil {
		return err
	}
	if len(evidence.SourceBindings) != len(plan.SourceArtifacts) {
		return fmt.Errorf("derivation replay source binding cardinality differs from its plan")
	}
	for index, binding := range evidence.SourceBindings {
		source := plan.SourceArtifacts[index]
		if !binding.Verified || binding.ID != source.ID || !filepath.IsAbs(binding.Path) || binding.ExpectedSHA256 != source.Digest || binding.ObservedSHA256 != source.Digest || !validDigest(binding.ObservedSHA256) {
			return fmt.Errorf("derivation source binding %d is inconsistent", index)
		}
	}
	expectedTools, err := derivationPlanTools(plan)
	if err != nil || len(evidence.ToolBindings) != len(expectedTools) {
		return fmt.Errorf("derivation replay tool binding cardinality is inconsistent")
	}
	for index, binding := range evidence.ToolBindings {
		tool := expectedTools[index]
		if !binding.Verified || binding.ID != tool.Name || binding.Path != tool.Path || binding.Version != tool.Version || binding.ExpectedSHA256 != tool.Digest || binding.ObservedSHA256 != tool.Digest || !validDigest(binding.ObservedSHA256) {
			return fmt.Errorf("derivation tool binding %d is inconsistent", index)
		}
	}
	for index := range evidence.Runs {
		if err := validateDerivationRun(plan, evidence.Runs[index], fmt.Sprintf("repetition-%d", index+1)); err != nil {
			return err
		}
	}
	if evidence.Runs[0].Isolation.IsolatedRoot == evidence.Runs[1].Isolation.IsolatedRoot || !bytes.Equal(evidence.Runs[0].IR, evidence.Runs[1].IR) || !bytes.Equal(evidence.Runs[0].DecoderOutput, evidence.Runs[1].DecoderOutput) {
		return fmt.Errorf("derivation repetitions are not independent and byte-identical")
	}
	return nil
}

func validateDerivationPlanRecord(plan DerivationReplayPlan) error {
	graph := plan.Graph
	if plan.ID == "" || plan.Workspace.ID == "" || plan.Workspace.State != semanticir.WorkspaceSolutionNewTests || !filepath.IsAbs(plan.Workspace.Root) || filepath.Clean(plan.Workspace.Root) != plan.Workspace.Root || graph.WorkspaceTreeDigest != plan.Workspace.TreeSHA256 || len(graph.IR) == 0 || graph.IRDigest != digestBytes(graph.IR) || !validDigest(graph.IRDigest) || len(graph.DecoderOutput) == 0 || graph.DecoderOutputDigest != digestBytes(graph.DecoderOutput) || !validDigest(graph.DecoderOutputDigest) || !validDigest(graph.SourceDigest) {
		return fmt.Errorf("derivation replay plan has incomplete graph/workspace bindings")
	}
	if len(plan.SourceArtifacts) == 0 {
		return fmt.Errorf("derivation replay plan has no sources")
	}
	bound := false
	seenSourceIDs, seenSourcePaths := map[string]bool{}, map[string]bool{}
	for _, source := range plan.SourceArtifacts {
		if source.ID == "" || source.Path == "" || !validDigest(source.Digest) || seenSourceIDs[source.ID] || seenSourcePaths[source.Path] {
			return fmt.Errorf("derivation replay plan has an invalid source")
		}
		seenSourceIDs[source.ID], seenSourcePaths[source.Path] = true, true
		bound = bound || source.Digest == graph.SourceDigest
	}
	if !bound || graph.Provenance.ArtifactDigest != graph.SourceDigest || !seenSourceIDs[graph.Provenance.ArtifactID] {
		return fmt.Errorf("derivation graph source is detached from its plan")
	}
	if err := validateDerivationEnvironment(graph.Environment, graph.EnvironmentDigest); err != nil {
		return err
	}
	if err := validateDerivationDecoder(graph); err != nil {
		return err
	}
	if diagnostics := semanticir.ValidateProbeSteps(graph.DerivationSteps, graph.Provenance); semanticir.HasErrors(diagnostics) {
		return fmt.Errorf("derivation steps are invalid: %s", diagnostics[0].Message)
	}
	if diagnostics := semanticir.ValidateProbeSteps(graph.DecoderSteps, graph.Provenance); semanticir.HasErrors(diagnostics) {
		return fmt.Errorf("decoder steps are invalid: %s", diagnostics[0].Message)
	}
	irSteps, decoderSteps := 0, 0
	primaryTool := false
	seenStepIDs := map[string]bool{}
	seenOutputIDs := map[string]bool{}
	seenOutputPaths := map[string]bool{}
	for phaseIndex, steps := range [][]semanticir.ProbeStep{graph.DerivationSteps, graph.DecoderSteps} {
		for _, step := range steps {
			if seenStepIDs[step.ID] {
				return fmt.Errorf("derivation and decoder step IDs are not globally unique")
			}
			seenStepIDs[step.ID] = true
			if step.Kind == semanticir.ProbeStepCleanup || !reflect.DeepEqual(step.Environment, graph.Environment) || step.EnvironmentDigest != graph.EnvironmentDigest || step.SignalExtractor.Kind != semanticir.ProbeSignalNone || step.ExpectedSignalDigest != digestBytes(nil) {
				return fmt.Errorf("derivation step %q violates exact environment/signal policy", step.ID)
			}
			primaryTool = primaryTool || reflect.DeepEqual(step.Tool, graph.Tool)
			if step.Kind == semanticir.ProbeStepRun {
				if phaseIndex == 0 && step.ExpectedStdoutDigest == graph.IRDigest {
					irSteps++
				}
				if phaseIndex == 1 && step.ExpectedStdoutDigest == graph.DecoderOutputDigest {
					decoderSteps++
				}
			}
			for _, output := range step.Outputs {
				if output.ExistedBefore || output.BeforeDigest != "" {
					return fmt.Errorf("derivation output %q is not declared fresh", output.ID)
				}
				path, err := declaredReplayOutputPath(plan.Workspace.Root, step.WorkingDirectory, output.Path)
				if err != nil {
					return fmt.Errorf("derivation output %q path is invalid: %w", output.ID, err)
				}
				if seenOutputIDs[output.ID] || seenOutputPaths[path] {
					return fmt.Errorf("derivation generated output identities and paths are not globally unique")
				}
				seenOutputIDs[output.ID], seenOutputPaths[path] = true, true
			}
		}
	}
	if irSteps != 1 || decoderSteps != 1 || !primaryTool {
		return fmt.Errorf("derivation graph lacks unambiguous primary-tool IR/decoder runs")
	}
	if _, err := derivationPlanTools(plan); err != nil {
		return err
	}
	return nil
}

func derivationPlanTools(plan DerivationReplayPlan) ([]semanticir.ToolRef, error) {
	byKey := map[string]semanticir.ToolRef{}
	add := func(tool semanticir.ToolRef) error {
		if tool == (semanticir.ToolRef{}) {
			return nil
		}
		if tool.Name == "" || tool.Version == "" || !filepath.IsAbs(tool.Path) || filepath.Clean(tool.Path) != tool.Path || !validDigest(tool.Digest) {
			return fmt.Errorf("derivation plan has an invalid frozen tool")
		}
		key := tool.Name + "\x00" + tool.Path
		if prior, exists := byKey[key]; exists && !reflect.DeepEqual(prior, tool) {
			return fmt.Errorf("derivation plan has inconsistent tool binding %q", tool.Name)
		}
		byKey[key] = tool
		return nil
	}
	if err := add(plan.Graph.Tool); err != nil {
		return nil, err
	}
	for _, steps := range [][]semanticir.ProbeStep{plan.Graph.DerivationSteps, plan.Graph.DecoderSteps} {
		for _, step := range steps {
			if err := add(step.Tool); err != nil {
				return nil, err
			}
		}
	}
	result := make([]semanticir.ToolRef, 0, len(byKey))
	for _, tool := range byKey {
		result = append(result, tool)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name+"\x00"+result[i].Path < result[j].Name+"\x00"+result[j].Path
	})
	return result, nil
}

func validateDerivationRun(plan DerivationReplayPlan, run DerivationReplayRun, id string) error {
	irStepID, decoderStepID := derivationTargetStepIDs(plan.Graph)
	if run.ID != id || !run.Complete || run.Error != "" || !run.ToolsVerifiedAfterRun || run.IRStepID != irStepID || run.DecoderStepID != decoderStepID || !bytes.Equal(run.IR, plan.Graph.IR) || run.IRSHA256 != plan.Graph.IRDigest || !bytes.Equal(run.DecoderOutput, plan.Graph.DecoderOutput) || run.DecoderOutputSHA256 != plan.Graph.DecoderOutputDigest || len(run.DerivationSteps) != len(plan.Graph.DerivationSteps) || len(run.DecoderSteps) != len(plan.Graph.DecoderSteps) {
		return fmt.Errorf("derivation run %q is incomplete or its IR/decoder output differs", run.ID)
	}
	if run.Isolation.OriginalRoot == "" || !filepath.IsAbs(run.Isolation.OriginalRoot) || run.Isolation.IsolatedRoot == "" || !filepath.IsAbs(run.Isolation.IsolatedRoot) || run.Isolation.OriginalRoot == run.Isolation.IsolatedRoot || !run.Isolation.IsolatedRemoved || !run.Isolation.OriginalIntact || run.Isolation.Error != "" || run.Isolation.ExpectedSHA256 != plan.Workspace.TreeSHA256 || run.Isolation.OriginalBeforeSHA256 != plan.Workspace.TreeSHA256 || run.Isolation.OriginalAfterSHA256 != plan.Workspace.TreeSHA256 || run.Isolation.CopyBeforeSHA256 != plan.Workspace.TreeSHA256 || !validDigest(run.Isolation.CopyAfterSHA256) {
		return fmt.Errorf("derivation run %q has incomplete isolation evidence", run.ID)
	}
	declaredOutputs := map[string]struct {
		output   semanticir.ProbeOutput
		producer string
	}{}
	for _, steps := range [][]semanticir.ProbeStep{plan.Graph.DerivationSteps, plan.Graph.DecoderSteps} {
		for _, step := range steps {
			for _, output := range step.Outputs {
				declaredOutputs[output.ID] = struct {
					output   semanticir.ProbeOutput
					producer string
				}{output: output, producer: step.ID}
			}
		}
	}
	if len(run.GeneratedOutputs) != len(declaredOutputs) {
		return fmt.Errorf("derivation run %q generated output cardinality differs", run.ID)
	}
	outputByID := map[string]ExhaustiveGeneratedOutputEvidence{}
	for _, output := range run.GeneratedOutputs {
		declared, exists := declaredOutputs[output.Output.ID]
		if !exists || !reflect.DeepEqual(output.Output, declared.output) || output.ProducerStepID != declared.producer || output.Output.ExistedBefore || !output.Fresh || !output.RemovedAfterRun || output.RemovalError != "" || output.SHA256 != output.Output.AfterDigest || !validDigest(output.SHA256) || !output.VerifiedAfterRun || output.AfterRunSHA256 != output.SHA256 {
			return fmt.Errorf("derivation run %q has invalid generated output evidence", run.ID)
		}
		if _, duplicate := outputByID[output.Output.ID]; duplicate {
			return fmt.Errorf("derivation run %q duplicates generated output evidence", run.ID)
		}
		outputByID[output.Output.ID] = output
	}
	for phaseIndex, commands := range [][]ExhaustiveReplayCommandEvidence{run.DerivationSteps, run.DecoderSteps} {
		steps := plan.Graph.DerivationSteps
		targetID, expected := run.IRStepID, plan.Graph.IR
		if phaseIndex == 1 {
			steps, targetID, expected = plan.Graph.DecoderSteps, run.DecoderStepID, plan.Graph.DecoderOutput
		}
		for index, command := range commands {
			step := steps[index]
			if !reflect.DeepEqual(command.Step, step) || command.StepSHA256 != mustProbeDigest(step) || !command.Passed || command.Error != "" || command.TimedOut || command.Interrupted || command.OutputTruncated || command.StartedAt.IsZero() || command.Duration <= 0 || command.ExitCode == nil || *command.ExitCode != step.ExpectedExitCode || command.StdoutSHA256 != step.ExpectedStdoutDigest || command.StdoutSHA256 != digestBytes(command.Stdout) || command.StderrSHA256 != step.ExpectedStderrDigest || command.StderrSHA256 != digestBytes(command.Stderr) || command.SignalSHA256 != step.ExpectedSignalDigest || command.SignalSHA256 != digestBytes(command.SignalValue) || !bytes.Equal(command.Stdout, func() []byte {
				if step.ID == targetID {
					return expected
				}
				return command.Stdout
			}()) {
				return fmt.Errorf("derivation run %q step %q is inconsistent", run.ID, step.ID)
			}
			if command.StdinSHA256 != digestBytes(step.Stdin) || command.EnvironmentSHA256 != step.EnvironmentDigest || !reflect.DeepEqual(command.Argv, step.Argv) || !command.FreshSignal || !command.SignalRemoved || len(command.SignalValue) != 0 {
				return fmt.Errorf("derivation run %q step %q invocation binding is inconsistent", run.ID, step.ID)
			}
			wantWorkDir, workErr := derivationEvidenceWorkDir(plan.Workspace.Root, run.Isolation.IsolatedRoot, step.WorkingDirectory)
			if workErr != nil || command.WorkingDirectory != wantWorkDir || !pathWithin(run.Isolation.IsolatedRoot, command.WorkingDirectory) || !filepath.IsAbs(command.ExecutablePath) {
				return fmt.Errorf("derivation run %q step %q path binding is inconsistent", run.ID, step.ID)
			}
			if step.GeneratedExecutableID != "" {
				output, exists := outputByID[step.GeneratedExecutableID]
				wantExecutable := filepath.Join(run.Isolation.IsolatedRoot, filepath.FromSlash(output.Path))
				if !exists || command.ExecutablePath != wantExecutable || !output.VerifiedBeforeRun || !output.VerifiedAfterRun || output.BeforeRunSHA256 != output.SHA256 || output.AfterRunSHA256 != output.SHA256 || command.ExecutableSHA256 != output.SHA256 {
					return fmt.Errorf("derivation run %q generated executable binding is inconsistent", run.ID)
				}
			} else if command.ExecutableSHA256 != step.Tool.Digest {
				return fmt.Errorf("derivation run %q frozen executable binding is inconsistent", run.ID)
			}
		}
	}
	return nil
}

func derivationTargetStepIDs(graph semanticir.CompilerSemanticGraph) (string, string) {
	irStepID, decoderStepID := "", ""
	for _, step := range graph.DerivationSteps {
		if step.Kind == semanticir.ProbeStepRun && step.ExpectedStdoutDigest == graph.IRDigest {
			irStepID = step.ID
		}
	}
	for _, step := range graph.DecoderSteps {
		if step.Kind == semanticir.ProbeStepRun && step.ExpectedStdoutDigest == graph.DecoderOutputDigest {
			decoderStepID = step.ID
		}
	}
	return irStepID, decoderStepID
}

// declaredReplayOutputPath normalizes only the declaration. It deliberately
// does not consult the mutable workspace, so certificate validation can still
// reject path aliases after the disposable copy has been removed.
func declaredReplayOutputPath(root, workDir, output string) (string, error) {
	if root == "" || !filepath.IsAbs(root) || output == "" || filepath.IsAbs(output) {
		return "", fmt.Errorf("workspace root must be absolute and output must be relative")
	}
	workRelative := filepath.FromSlash(workDir)
	if filepath.IsAbs(workRelative) {
		var err error
		workRelative, err = filepath.Rel(root, workRelative)
		if err != nil {
			return "", err
		}
	}
	workRelative = filepath.Clean(workRelative)
	if filepath.IsAbs(workRelative) || workRelative == ".." || strings.HasPrefix(workRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("working directory escapes workspace")
	}
	path := filepath.Clean(filepath.Join(workRelative, filepath.FromSlash(output)))
	if filepath.IsAbs(path) || path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generated output escapes workspace")
	}
	return filepath.ToSlash(path), nil
}

func derivationEvidenceWorkDir(originalRoot, isolatedRoot, declared string) (string, error) {
	relative := filepath.FromSlash(declared)
	if filepath.IsAbs(declared) {
		var err error
		relative, err = filepath.Rel(originalRoot, declared)
		if err != nil {
			return "", err
		}
	}
	relative = filepath.Clean(relative)
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("working directory escapes workspace")
	}
	return filepath.Join(isolatedRoot, relative), nil
}
