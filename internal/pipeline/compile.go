package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HyperMarble/ray/internal/semanticir"
	"github.com/HyperMarble/ray/internal/speccompiler"
	"github.com/HyperMarble/ray/internal/taskbundle"
)

// compileSkeleton is the only proof-path reader of spec.md. It compiles the
// exact frozen Spec and instruction once, attaches the independently frozen
// environment model, and returns only typed Semantic IR to later proof stages.
// Diagnostics may read the frozen source separately, but may never mutate the
// compiled task or manufacture reference/Test semantics from it.
func compileSkeleton(ctx context.Context, root string, cfg config, manifest taskbundle.Manifest, environment *semanticir.EnvironmentModel) (*semanticir.Task, []string) {
	spec, err := manifestArtifact(manifest, cfg.SpecArtifactID, semanticir.ArtifactSpec)
	if err != nil {
		return nil, []string{err.Error()}
	}
	instruction, err := manifestArtifact(manifest, cfg.InstructionArtifactID, semanticir.ArtifactInstruction)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if manifest.RequiredInputs.SpecArtifactID != spec.ID || manifest.RequiredInputs.InstructionArtifactID != instruction.ID {
		return nil, []string{"strict Spec/instruction bindings differ from the frozen required-input roles"}
	}
	specSource, err := readFrozenArtifact(root, spec)
	if err != nil {
		return nil, []string{err.Error()}
	}
	instructionSource, err := readFrozenArtifact(root, instruction)
	if err != nil {
		return nil, []string{err.Error()}
	}
	// The frozen reference solution is the second provenance artifact: a row's
	// Evidence cell may anchor into it with `reference:<span>` (evidence-rule.md).
	// Without it every reference-anchored row would fail here after passing
	// spec-lint, and the two tools must accept the same specs.
	var reference semanticir.ArtifactRef
	var referenceSource []byte
	if ids := manifest.RequiredInputs.SolutionArtifactIDs; len(ids) > 0 {
		reference, err = manifestSolutionArtifact(manifest, ids[0])
		if err != nil {
			return nil, []string{err.Error()}
		}
		referenceSource, err = readFrozenArtifact(root, reference)
		if err != nil {
			return nil, []string{err.Error()}
		}
	}
	task, diagnostics := speccompiler.Compile(ctx, speccompiler.Request{
		TaskID: cfg.TaskID, Artifact: spec, Source: specSource,
		Instruction: instruction, InstructionSource: instructionSource,
		Reference: reference, ReferenceSource: referenceSource,
	})
	if task == nil || semanticir.HasErrors(diagnostics) {
		if len(diagnostics) == 0 {
			return nil, []string{"strict Spec compiler returned no task and no diagnostic"}
		}
		return nil, diagnosticStrings(diagnostics)
	}
	if task.Spec != spec || task.Instruction != instruction {
		return nil, []string{"strict Spec compiler returned detached Spec or instruction evidence"}
	}
	if diagnostics := semanticir.ValidateSpecIRDigest(task); semanticir.HasErrors(diagnostics) {
		return nil, append([]string{"strict Spec compiler omitted or detached canonical Spec IR"}, diagnosticStrings(diagnostics)...)
	}
	task.Environment = environment
	return task, nil
}

func attachArtifacts(task *semanticir.Task, records []translationRecord, kind semanticir.ArtifactKind) []string {
	if task == nil {
		return []string{"strict Spec compiler returned a nil task"}
	}
	for _, record := range records {
		if record.model.Kind != kind {
			continue
		}
		if mergeDiagnostics := task.AddArtifactScope(record.request, record.model); semanticir.HasErrors(mergeDiagnostics) {
			return diagnosticStrings(mergeDiagnostics)
		}
	}
	return nil
}

func validateFinalTask(task *semanticir.Task) []string {
	if task == nil {
		return []string{"strict Spec compiler returned a nil task"}
	}
	if validation := task.Validate(); semanticir.HasErrors(validation) {
		return diagnosticStrings(validation)
	}
	return nil
}

func readFrozenArtifact(root string, artifact semanticir.ArtifactRef) ([]byte, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.Path)))
	if err != nil {
		return nil, fmt.Errorf("read frozen artifact %q: %w", artifact.ID, err)
	}
	if err := semanticir.VerifyArtifact(artifact, content); err != nil {
		return nil, err
	}
	return content, nil
}

// manifestSolutionArtifact resolves the frozen reference solution, whose
// manifest kind is "solution" for a patch role and "code" for a plain file.
func manifestSolutionArtifact(manifest taskbundle.Manifest, id string) (semanticir.ArtifactRef, error) {
	for _, artifact := range manifest.Artifacts {
		if artifact.ID != id {
			continue
		}
		if artifact.Kind != "solution" && artifact.Kind != "code" {
			return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q has kind %q, want solution or code", id, artifact.Kind)
		}
		return semanticir.ArtifactRef{ID: id, Kind: semanticir.ArtifactCode, Path: artifact.Path, Digest: artifact.SHA256}, nil
	}
	return semanticir.ArtifactRef{}, fmt.Errorf("frozen artifact %q is absent from manifest", id)
}
