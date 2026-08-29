package proof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/HyperMarble/hyperray/internal/semanticir"
)

// Replay independently checks and reruns one compiler-path proof. The exact
// frozen prover binary, query, invocation, environment binding, timeout, and
// byte-for-byte solver output all participate; a frontend's ProofProved flag
// is never accepted on its own.
func Replay(ctx context.Context, replay semanticir.ReplayableProof, expected semanticir.SolverResult, environment *semanticir.EnvironmentModel) error {
	if ctx == nil {
		ctx = context.Background()
	}
	diagnostics := semanticir.ValidateReplayableProof(replay, expected, semanticir.Provenance{})
	if semanticir.HasErrors(diagnostics) {
		messages := make([]string, 0, len(diagnostics))
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == semanticir.SeverityError {
				messages = append(messages, diagnostic.Message)
			}
		}
		return fmt.Errorf("invalid replayable proof: %s", strings.Join(messages, "; "))
	}
	if environment == nil {
		return fmt.Errorf("replayable proof has no frozen environment")
	}
	if !containsToolRef(environment.Tools, replay.Prover) {
		return fmt.Errorf("replayable prover %q is not an exact frozen environment ToolRef", replay.Prover.Name)
	}
	environmentBound := replay.EnvironmentDigest == environment.ConfigDigest
	for _, command := range environment.Commands {
		if command.EnvironmentDigest == replay.EnvironmentDigest {
			environmentBound = true
			break
		}
	}
	if !environmentBound {
		return fmt.Errorf("replayable proof environment digest is not frozen in the environment model")
	}
	if err := verifyToolBinary(replay.Prover); err != nil {
		return err
	}
	if !strings.EqualFold(replay.Prover.Name, "z3") {
		return fmt.Errorf("replayable SMT proof uses unsupported prover %q; exact Z3 is required", replay.Prover.Name)
	}
	if !replay.ClearEnvironment || !replay.KillProcessGroup {
		return fmt.Errorf("replayable proof must clear ambient environment and kill its process group on cancellation")
	}
	if strings.TrimSpace(replay.WorkingDirectory) == "" || !strings.HasPrefix(replay.WorkingDirectory, "/") {
		return fmt.Errorf("replayable proof working directory is not absolute")
	}
	encodedEnvironment, err := semanticir.Digest(replay.Environment)
	if err != nil || encodedEnvironment != replay.EnvironmentDigest {
		return fmt.Errorf("replayable proof environment entries do not match their canonical digest")
	}
	commandEnvironment := make([]string, len(replay.Environment))
	previousName := ""
	for i, variable := range replay.Environment {
		if strings.TrimSpace(variable.Name) == "" || strings.Contains(variable.Name, "=") || strings.ContainsRune(variable.Name, '\x00') || strings.ContainsRune(variable.Value, '\x00') || (i > 0 && variable.Name <= previousName) {
			return fmt.Errorf("replayable proof environment is not a strictly sorted unique name/value list")
		}
		previousName = variable.Name
		commandEnvironment[i] = variable.Name + "=" + variable.Value
	}
	versionStdout, versionStderr, versionErr := runHermetic(ctx, replay.Prover.Path, []string{"-version"}, nil, replay.WorkingDirectory, commandEnvironment, replay.TimeoutMillis)
	if versionErr != nil || len(versionStderr) != 0 || strings.TrimSpace(string(versionStdout)) != strings.TrimSpace(replay.Prover.Version) {
		return fmt.Errorf("replayable Z3 version differs from frozen identity: output=%q stderr=%q err=%v", strings.TrimSpace(string(versionStdout)), strings.TrimSpace(string(versionStderr)), versionErr)
	}

	timeout := time.Duration(replay.TimeoutMillis) * time.Millisecond
	replayCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(replayCtx, replay.Prover.Path, replay.Argv...)
	command.Stdin = bytes.NewReader(replay.Query)
	// Never inherit ambient variables into a proof replay. The stable IR will
	// supply the exact frozen entries; until then only hermetic solvers that
	// need no environment are replayable.
	command.Env = commandEnvironment
	command.Dir = replay.WorkingDirectory
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if replayCtx.Err() != nil {
			return fmt.Errorf("replayable proof timed out or was cancelled: %w", replayCtx.Err())
		}
		return fmt.Errorf("run replayable prover: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() != 0 {
		return fmt.Errorf("replayable prover wrote diagnostics: %s", strings.TrimSpace(stderr.String()))
	}
	if !bytes.Equal(stdout.Bytes(), replay.SolverOutput) {
		return fmt.Errorf("replayable prover output differs from the frozen byte transcript")
	}
	fields := strings.Fields(stdout.String())
	if len(fields) == 0 || semanticir.SolverResult(fields[0]) != expected {
		return fmt.Errorf("replayed solver result is %q, want %q", firstField(fields), expected)
	}
	return nil
}

func verifyToolBinary(tool semanticir.ToolRef) error {
	if !filepath.IsAbs(tool.Path) {
		return fmt.Errorf("frozen tool path %q is not absolute", tool.Path)
	}
	file, err := os.Open(tool.Path)
	if err != nil {
		return fmt.Errorf("open frozen tool %q: %w", tool.Path, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash frozen tool %q: %w", tool.Path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close frozen tool %q: %w", tool.Path, closeErr)
	}
	digest := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if digest != tool.Digest {
		return fmt.Errorf("frozen tool %q digest mismatch: got %s, expected %s", tool.Name, digest, tool.Digest)
	}
	return nil
}

func firstField(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
