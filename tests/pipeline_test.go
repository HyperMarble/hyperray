package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/internal/pipeline"
)

func TestPipelineRejectsMissingFrozenTask(t *testing.T) {
	result := pipeline.Run(context.Background(), pipeline.Request{Root: t.TempDir()})
	if result.Verdict != pipeline.ProofBlocked || len(result.Stages) != 1 {
		t.Fatalf("missing task did not fail closed: %+v", result)
	}
	if result.Stages[0].Name != pipeline.StageFreeze || result.Stages[0].Status != pipeline.StageBlocked {
		t.Fatalf("missing task blocked at wrong stage: %+v", result.Stages)
	}
}

func TestPipelineRejectsNilContext(t *testing.T) {
	result := pipeline.Run(nil, pipeline.Request{Root: t.TempDir()})
	if result.Verdict != pipeline.ProofBlocked || len(result.Stages) != 1 ||
		result.Stages[0].Name != pipeline.StageFreeze ||
		!strings.Contains(strings.Join(result.Stages[0].Diagnostic, "\n"), "nil execution context") {
		t.Fatalf("nil context did not fail closed: %+v", result)
	}
}

func TestPipelineIgnoresMutationForProof(t *testing.T) {
	// The sole production request surface accepts frozen paths only. It has no
	// hook for a mutation score, sampled result, or caller-supplied verdict.
	request := pipeline.Request{Root: t.TempDir(), ConfigPath: "hyperray.toml", CertificatePath: "certificate.json"}
	result := pipeline.Run(context.Background(), request)
	if result.Verdict != pipeline.ProofBlocked || result.Successful() {
		t.Fatalf("caller-controlled advisory state reached success: %+v", result)
	}
}
