package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HyperMarble/hyperray/internal/certificate"
)

const e2eTimeout = 4 * time.Minute

type cliResult struct {
	root   string
	stdout string
	stderr string
	err    error
}

type jcodeSourceMetadata struct {
	Schema              string `json:"schema"`
	BaseCommit          string `json:"base_commit"`
	SolutionCommit      string `json:"solution_commit"`
	SourcePath          string `json:"source_path"`
	BaseSHA256          string `json:"base_sha256"`
	SolutionSHA256      string `json:"solution_sha256"`
	SolutionPatchSHA256 string `json:"solution_patch_sha256"`
	TestsPatchSHA256    string `json:"tests_patch_sha256"`
}

func TestE2ERealTaskJcodeProvenanceSplitReconstructsRealSource(t *testing.T) {
	repo := e2eRepositoryRoot(t)
	root := filepath.Join(repo, "testdata", "e2e", "real-rust-jcode-picker-negative")
	metadataSource, err := os.ReadFile(filepath.Join(root, "provenance", "base-metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata jcodeSourceMetadata
	if err := json.Unmarshal(metadataSource, &metadata); err != nil {
		t.Fatalf("decode jcode source metadata: %v", err)
	}
	if metadata.Schema != "hyperray.task-source/v1" || metadata.BaseCommit != "95087d57b3d5b5dd02c64a12c44e149d4426abad" || metadata.SolutionCommit != "f85c2d596f02d943dbb72e45a88e4e6071c9f8b7" {
		t.Fatalf("unexpected jcode repository identity: %+v", metadata)
	}
	assertE2EFileSHA256(t, filepath.Join(root, "instruction.md"), "bd62bf12f53916f13be9007e595bd3823223765274688fd9cd772dd577d339d6")
	baseRoot := filepath.Join(root, "source", "base")
	solutionRoot := filepath.Join(root, "source", "solution")
	baseSource := filepath.Join(baseRoot, filepath.FromSlash(metadata.SourcePath))
	solutionSource := filepath.Join(solutionRoot, filepath.FromSlash(metadata.SourcePath))
	assertE2EFileSHA256(t, baseSource, metadata.BaseSHA256)
	assertE2EFileSHA256(t, solutionSource, metadata.SolutionSHA256)

	solutionPatch := filepath.Join(root, "patches", "solution.patch")
	assertE2EFileSHA256(t, solutionPatch, metadata.SolutionPatchSHA256)
	testsPatch := filepath.Join(root, "patches", "tests.patch")
	assertE2EFileSHA256(t, testsPatch, metadata.TestsPatchSHA256)
	for relative, digest := range map[string]string{
		"source/base/src/lib.rs":               "a95074a7dfbc4560480feec2747c09a1ba8659d0af7649cfad3e83016e09d1bd",
		"source/base/src/tui/mod.rs":           "787b1879a06cacadfd6c2ae439cc2d0ecd64e48351fcc7a9813fcf6cfdcae6c1",
		"source/base/src/tui/ui.rs":            "6258d21757703addb85a3ec4d870a8048875215a6116d08cf1b6caf84a640a2f",
		"source/solution/src/lib.rs":           "a95074a7dfbc4560480feec2747c09a1ba8659d0af7649cfad3e83016e09d1bd",
		"source/solution/src/tui/mod.rs":       "787b1879a06cacadfd6c2ae439cc2d0ecd64e48351fcc7a9813fcf6cfdcae6c1",
		"source/solution/src/tui/ui.rs":        "6258d21757703addb85a3ec4d870a8048875215a6116d08cf1b6caf84a640a2f",
		"environment/Cargo.toml":               "10fa041aea4c2d816c215c6a9f4595dab12b642a5eebfcd3f77430650e7bfa52",
		"environment/Cargo.lock":               "0732e41972891a86886014b65c27bc3f8df291738f952a9623a29708bd16b798",
		"environment/.github/workflows/ci.yml": "6b024a0193fc885e5f890b13fd2a35b1f8dc0adfcb88ab47ff6e2a8dd9be7ea3",
	} {
		assertE2EFileSHA256(t, filepath.Join(root, filepath.FromSlash(relative)), digest)
	}
	patchSource, err := os.ReadFile(solutionPatch)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"#[cfg(test)]", "#[test]", "assert_eq!", "assert!("} {
		if bytes.Contains(patchSource, []byte(forbidden)) {
			t.Fatalf("test-blind solution patch leaks verifier construct %q", forbidden)
		}
	}

	replayRoot := filepath.Join(t.TempDir(), "replay")
	if err := os.CopyFS(replayRoot, os.DirFS(baseRoot)); err != nil {
		t.Fatalf("copy frozen base snapshot: %v", err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required to replay the real task patches")
	}
	for _, patchPath := range []string{
		testsPatch,
		solutionPatch,
	} {
		command := exec.Command(gitPath, "apply", "--whitespace=nowarn", patchPath)
		command.Dir = replayRoot
		command.Env = []string{"PATH=/usr/bin:/bin"}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("replay %s: %v\n%s", patchPath, err, output)
		}
	}
	replayed, err := os.ReadFile(filepath.Join(replayRoot, filepath.FromSlash(metadata.SourcePath)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(solutionSource)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replayed, want) {
		t.Fatal("ordered test+solution patch replay does not reconstruct the exact upstream solution source")
	}
	for _, expectation := range []string{
		`assert_eq!(picker_row_marker(true, false, true), "⚠");`,
		`assert_eq!(picker_row_marker(true, false, true), "▸");`,
	} {
		if !bytes.Contains(replayed, []byte(expectation)) {
			t.Fatalf("faithful handwritten verifier lost contradictory expectation %q", expectation)
		}
	}
}

func runRealTask(t *testing.T, subcommand, fixture string) cliResult {
	t.Helper()
	repo := e2eRepositoryRoot(t)
	fixtureRoot := filepath.Join(repo, "testdata", "e2e", fixture)
	if info, err := os.Stat(fixtureRoot); err != nil || !info.IsDir() {
		t.Fatalf("real task fixture %q is unavailable: %v", fixture, err)
	}

	taskRoot := filepath.Join(t.TempDir(), fixture)
	if err := os.CopyFS(taskRoot, os.DirFS(fixtureRoot)); err != nil {
		t.Fatalf("copy real task fixture: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "hyperray")
	build := exec.Command("go", "build", "-o", binary, "./cmd/hyperray")
	build.Dir = repo
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build production CLI: %v\n%s", err, output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e2eTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, binary, subcommand, taskRoot)
	command.Dir = repo
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("production CLI exceeded %s\nstdout:\n%s\nstderr:\n%s", e2eTimeout, stdout.String(), stderr.String())
	}
	return cliResult{root: taskRoot, stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func assertVerified(t *testing.T, result cliResult) {
	t.Helper()
	if result.err != nil {
		t.Fatalf("production CLI did not verify: %v\nstdout:\n%s\nstderr:\n%s", result.err, result.stdout, result.stderr)
	}
	if got := terminalLine(result.stdout); got != "VERIFIED" {
		t.Fatalf("terminal verdict = %q, want VERIFIED\nstdout:\n%s", got, result.stdout)
	}
	previousStage := -1
	for _, stage := range []string{
		"freeze: complete", "compile-spec: complete", "translate: complete", "compile-test-ir: complete",
		"proof-reference-within-spec: complete", "proof-tests-pass-within-spec: complete",
		"proof-spec-within-tests-pass: complete", "confirm-counterexamples: complete",
		"certificate: complete",
	} {
		index := strings.Index(result.stdout, stage+"\n")
		if index < 0 {
			t.Fatalf("missing mandatory completed stage %q\nstdout:\n%s", stage, result.stdout)
		}
		if index <= previousStage {
			t.Fatalf("mandatory stage %q is out of order\nstdout:\n%s", stage, result.stdout)
		}
		previousStage = index
	}
	cert := readE2ECertificate(t, result.root)
	if cert.Verdict != certificate.Verified {
		t.Fatalf("certificate verdict = %q, want VERIFIED", cert.Verdict)
	}
	if len(cert.Document.Counterexamples) != 0 {
		t.Fatalf("VERIFIED certificate contains counterexamples: %+v", cert.Document.Counterexamples)
	}
	for _, proof := range cert.Document.Proofs {
		if proof.Status != certificate.ProofProved {
			t.Fatalf("proof %s = %s, want PROVED", proof.Obligation, proof.Status)
		}
	}
}

func assertRefuted(t *testing.T, result cliResult, obligation certificate.ProofObligation) {
	t.Helper()
	if result.err == nil {
		t.Fatalf("negative task exited successfully\nstdout:\n%s", result.stdout)
	}
	if got := terminalLine(result.stdout); got != "NOT VERIFIED" {
		t.Fatalf("terminal verdict = %q, want NOT VERIFIED\nstdout:\n%s\nstderr:\n%s", got, result.stdout, result.stderr)
	}
	for _, stage := range []string{
		"freeze: complete", "compile-spec: complete", "translate: complete", "compile-test-ir: complete",
		"proof-" + string(obligation) + ": refuted", "confirm-counterexamples: complete", "certificate: complete",
	} {
		if !strings.Contains(result.stdout, stage+"\n") {
			t.Fatalf("missing negative-task stage %q\nstdout:\n%s", stage, result.stdout)
		}
	}
	cert := readE2ECertificate(t, result.root)
	if cert.Verdict != certificate.NotVerified {
		t.Fatalf("certificate verdict = %q, want NOT VERIFIED", cert.Verdict)
	}
	found := false
	for _, counterexample := range cert.Document.Counterexamples {
		if counterexample.Obligation != obligation {
			continue
		}
		found = true
		if counterexample.ID == "" || counterexample.WitnessDigest == "" {
			t.Fatalf("%s counterexample lacks stable ID/digest: %+v", obligation, counterexample)
		}
		if len(counterexample.Witness.Choices) == 0 {
			t.Fatalf("%s counterexample lacks a concrete behavior vector", obligation)
		}
		if counterexample.Confirmation.Status != "CONFIRMED" {
			t.Fatalf("%s counterexample was not confirmed: %+v", obligation, counterexample.Confirmation)
		}
	}
	if !found {
		t.Fatalf("certificate has no %s witness: %+v", obligation, cert.Document.Counterexamples)
	}
}

func assertE2EProofBlocked(t *testing.T, result cliResult, stages ...string) {
	t.Helper()
	if result.err == nil {
		t.Fatalf("PROOF BLOCKED task exited successfully\nstdout:\n%s", result.stdout)
	}
	if got := terminalLine(result.stdout); got != "PROOF BLOCKED" {
		t.Fatalf("terminal verdict = %q, want PROOF BLOCKED\nstdout:\n%s\nstderr:\n%s", got, result.stdout, result.stderr)
	}
	for _, stage := range stages {
		if !strings.Contains(result.stdout, stage+"\n") {
			t.Fatalf("missing honest-block stage %q\nstdout:\n%s", stage, result.stdout)
		}
	}
}

func readE2ECertificate(t *testing.T, root string) certificate.Certificate {
	t.Helper()
	path := filepath.Join(root, "hyperray-certificate.json")
	cert, err := certificate.Read(path)
	if err != nil {
		t.Fatalf("read verified production certificate %s: %v", path, err)
	}
	return cert
}

func terminalLine(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func assertE2EFileSHA256(t *testing.T, path, expected string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if observed := hex.EncodeToString(digest[:]); observed != expected {
		t.Fatalf("SHA-256 %s = %s, want %s", path, observed, expected)
	}
}

func e2eRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate e2e test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(fmt.Errorf("resolve repository root %s: %w", root, err))
	}
	return root
}
