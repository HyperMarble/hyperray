package rust

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

func pinnedRustProver(ctx context.Context, request semanticir.FrontendRequest) (semanticir.ToolRef, []string, []semanticir.EnvironmentVariable, []semanticir.Diagnostic) {
	artifact := request.Artifact
	prover := request.Prover
	if prover.Name != "z3" || !filepath.IsAbs(prover.Path) || !semanticir.ValidDigest(prover.Digest) || prover.Version == "" {
		return semanticir.ToolRef{}, nil, nil, []semanticir.Diagnostic{diagnostic(artifact, sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Line: 1, Column: 1}}, semanticir.DiagnosticInvalidReference, "Rust proof requires the exact typed frozen z3 identity")}
	}
	path, err := filepath.EvalSymlinks(prover.Path)
	if err != nil || path != prover.Path {
		return semanticir.ToolRef{}, nil, nil, []semanticir.Diagnostic{diagnostic(artifact, sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Line: 1, Column: 1}}, semanticir.DiagnosticInvalidReference, "Rust prover path must be absolute, canonical, and directly executable")}
	}
	body, err := os.ReadFile(prover.Path)
	if err != nil {
		return semanticir.ToolRef{}, nil, nil, []semanticir.Diagnostic{diagnostic(artifact, sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Line: 1, Column: 1}}, semanticir.DiagnosticInvalidReference, "read z3: "+err.Error())}
	}
	if semanticir.DigestBytes(body) != prover.Digest {
		return semanticir.ToolRef{}, nil, nil, []semanticir.Diagnostic{diagnostic(artifact, sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Line: 1, Column: 1}}, semanticir.DiagnosticStaleArtifact, "frozen z3 digest does not match executable bytes")}
	}
	versionCommand := exec.CommandContext(ctx, prover.Path, "--version")
	configureRustCommand(versionCommand, request.Workspace)
	versionBytes, err := versionCommand.CombinedOutput()
	if err != nil || strings.TrimSpace(string(versionBytes)) != prover.Version {
		return semanticir.ToolRef{}, nil, nil, []semanticir.Diagnostic{diagnostic(artifact, sourceSpan{Start: sourcePos{Line: 1, Column: 1}, End: sourcePos{Line: 1, Column: 1}}, semanticir.DiagnosticInvalidReference, "read pinned z3 version")}
	}
	return prover, []string{"-in", "-smt2"}, append([]semanticir.EnvironmentVariable(nil), request.Workspace.Environment...), nil
}

func replayRustClaim(ctx context.Context, prover semanticir.ToolRef, argv []string, environment []semanticir.EnvironmentVariable, claim semanticir.ProofClaim, expected semanticir.SolverResult) (semanticir.ReplayableProof, error) {
	query, queryErr := semanticir.BuildProofQuery(claim)
	if queryErr != nil {
		return semanticir.ReplayableProof{}, fmt.Errorf("build canonical proof query: %w", queryErr)
	}
	environment = append([]semanticir.EnvironmentVariable(nil), environment...)
	environmentDigest, digestErr := semanticir.Digest(environment)
	if digestErr != nil {
		return semanticir.ReplayableProof{}, fmt.Errorf("digest frozen Rust proof environment: %w", digestErr)
	}
	proof := semanticir.ReplayableProof{Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: append([]byte(nil), query...), QueryDigest: semanticir.DigestBytes(query), Prover: prover, Argv: append([]string(nil), argv...), WorkingDirectory: "/", Environment: environment, EnvironmentDigest: environmentDigest, ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000, Result: expected, SubjectDigests: semanticir.ProofClaimSubjectDigests(claim)}
	proofCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(proofCtx, prover.Path, argv...)
	command.Env = make([]string, len(environment))
	for index, variable := range environment {
		command.Env[index] = variable.Name + "=" + variable.Value
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 250 * time.Millisecond
	command.Dir = proof.WorkingDirectory
	command.Stdin = strings.NewReader(string(query))
	output, err := command.CombinedOutput()
	if err != nil {
		return proof, fmt.Errorf("z3 replay failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	proof.SolverOutput = append([]byte(nil), output...)
	proof.SolverOutputDigest = semanticir.DigestBytes(output)
	fields := strings.Fields(string(output))
	if len(fields) == 0 || semanticir.SolverResult(fields[0]) != expected {
		return proof, fmt.Errorf("z3 returned %q, want %s", strings.TrimSpace(string(output)), expected)
	}
	return proof, nil
}

func rustCompilerPredicate(tool semanticir.ToolRef, irDigest, declarations, formula string, nodeIDs []string) semanticir.CompilerPredicate {
	declarationBytes := []byte(declarations)
	formulaBytes := []byte(formula)
	return semanticir.CompilerPredicate{Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarationBytes, DeclarationsDigest: semanticir.DigestBytes(declarationBytes), Formula: formulaBytes, FormulaDigest: semanticir.DigestBytes(formulaBytes), Tool: tool, IRDigest: irDigest, CompilerNodeIDs: append([]string(nil), nodeIDs...)}
}
