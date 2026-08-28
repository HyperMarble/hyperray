package cpp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/HyperMarble/ray/internal/semanticir"
)

// This file contains only exact tool binding and replay helpers used by
// scope-closure evidence. Compiler behavior authority for the supported pure
// free-function subset is exhaustive compiled execution in execution.go.
// In particular, no AST-derived outcome table is repackaged as an SMT proof.

func pinnedZ3(ctx context.Context, workspace semanticir.WorkspaceRef) (semanticir.ToolRef, error) {
	path, ok := workspaceLookPath("z3", workspace)
	if !ok {
		return semanticir.ToolRef{}, fmt.Errorf("z3 is absent from the frozen workspace PATH")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return semanticir.ToolRef{}, fmt.Errorf("read z3 prover: %w", err)
	}
	command := exec.CommandContext(ctx, path, "--version")
	configureCPPCommand(command, workspace)
	versionBytes, err := command.CombinedOutput()
	if err != nil {
		return semanticir.ToolRef{}, fmt.Errorf("query z3 version: %v: %s", err, versionBytes)
	}
	version := strings.TrimSpace(string(versionBytes))
	if version == "" {
		return semanticir.ToolRef{}, fmt.Errorf("z3 returned an empty version")
	}
	return semanticir.ToolRef{Name: "z3", Path: path, Digest: semanticir.DigestBytes(body), Version: version}, nil
}

func replaySMTProof(ctx context.Context, prover semanticir.ToolRef, workspace semanticir.WorkspaceRef, claim semanticir.ProofClaim) (semanticir.ReplayableProof, semanticir.SolverResult, error) {
	query, err := semanticir.CanonicalProofQuery(claim)
	if err != nil {
		return semanticir.ReplayableProof{}, semanticir.SolverUnknown, fmt.Errorf("construct canonical proof query: %w", err)
	}
	proofCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	argv := []string{"-in", "-smt2"}
	command := exec.CommandContext(proofCtx, prover.Path, argv...)
	// The query is self-contained. Its compiler/source/workspace identity is
	// bound in claim.Context, so replay remains valid after an isolated
	// candidate workspace has been removed.
	command.Dir = "/"
	configureCPPCommand(command, workspace)
	command.Stdin = strings.NewReader(string(query))
	output, err := command.Output()
	if err != nil {
		return semanticir.ReplayableProof{}, semanticir.SolverUnknown, fmt.Errorf("run z3: %w", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return semanticir.ReplayableProof{}, semanticir.SolverUnknown, fmt.Errorf("z3 returned empty output")
	}
	result := semanticir.SolverResult(fields[0])
	if result != semanticir.SolverSAT && result != semanticir.SolverUNSAT && result != semanticir.SolverUnknown {
		return semanticir.ReplayableProof{}, semanticir.SolverUnknown, fmt.Errorf("z3 returned unexpected result %q", fields[0])
	}
	environment := append([]semanticir.EnvironmentVariable(nil), workspace.Environment...)
	environmentDigest, digestErr := semanticir.Digest(environment)
	if digestErr != nil {
		return semanticir.ReplayableProof{}, semanticir.SolverUnknown, fmt.Errorf("digest proof environment: %w", digestErr)
	}
	proof := semanticir.ReplayableProof{
		Claim: claim, Logic: semanticir.ProofLogicSMTLIB2, Query: append([]byte(nil), query...), QueryDigest: semanticir.DigestBytes(query),
		Prover: prover, Argv: argv, WorkingDirectory: "/", Environment: environment, EnvironmentDigest: environmentDigest,
		ClearEnvironment: true, KillProcessGroup: true, TimeoutMillis: 5000,
		SolverOutput: append([]byte(nil), output...), SolverOutputDigest: semanticir.DigestBytes(output), Result: result,
		SubjectDigests: semanticir.ProofClaimSubjectDigests(claim),
	}
	return proof, result, nil
}

func compilerPredicate(tool semanticir.ToolRef, irDigest string, declarations, formula []byte, nodeIDs []string) semanticir.CompilerPredicate {
	declarations = append([]byte(nil), declarations...)
	formula = append([]byte(nil), formula...)
	return semanticir.CompilerPredicate{
		Logic: semanticir.ProofLogicSMTLIB2, Declarations: declarations,
		DeclarationsDigest: semanticir.DigestBytes(declarations), Formula: formula,
		FormulaDigest: semanticir.DigestBytes(formula), Tool: tool, IRDigest: irDigest,
		CompilerNodeIDs: append([]string(nil), nodeIDs...),
	}
}

func llvmSymbol(module, mangled, fallback string) string {
	candidates := []string{mangled, fallback}
	if strings.Contains(module, "target triple = \"arm64-apple-") && strings.HasPrefix(mangled, "__Z") {
		candidates = append([]string{mangled[1:]}, candidates...)
	}
	for _, candidate := range candidates {
		if _, ok := llvmFunctionBody(module, candidate); ok {
			return candidate
		}
	}
	return ""
}
