// Semantic resource tests require cause details and zero reports.
// They must not accept generic limits or partial command output.
package isla

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const semanticResourceTool = `#!/bin/sh
set -eu
program=
for argument in "$@"; do program=$argument; done
case "$(cat "$program")" in
deadline|simultaneous) printf '%s' 'stdout-data'; printf '%s' 'stderr-data' >&2; sleep 2 ;;
stdout) printf '%s' 'stdout-data' ;;
stderr) printf '%s' 'stderr-data' >&2 ;;
process) printf '%s' 'process-output'; printf '%s' 'process-diagnostic' >&2; exit 3 ;;
*) exit 0 ;;
esac
`

func TestSemanticResourceErrorDetails(t *testing.T) {
	cases := []struct {
		name, mode, detail string
		code               ErrorCode
		limit, seconds     uint64
		cancel             bool
	}{
		{"deadline", "deadline", "context: deadline exceeded", ResourceLimit, 64, 1, false},
		{"canceled", "stdout", "context: canceled", ResourceLimit, 64, 5, true},
		{"stdout cap", "stdout", "stdout: cap exceeded (retained 5 bytes, cap 5 bytes)", ResourceLimit, 5, 5, false},
		{"stderr cap", "stderr", "stderr: cap exceeded (retained 5 bytes, cap 5 bytes)", ResourceLimit, 5, 5, false},
		{"simultaneous", "simultaneous", "context: deadline exceeded; stdout: cap exceeded (retained 5 bytes, cap 5 bytes); stderr: cap exceeded (retained 5 bytes, cap 5 bytes)", ResourceLimit, 5, 1, false},
		{"process failure", "process", "process-diagnostic\nprocess-output", ProcessFail, 64, 5, false},
	}
	for index := range cases {
		testCase := cases[index]
		t.Run(testCase.name, func(t *testing.T) {
			output, err := runSemanticResourceCase(t, testCase.mode, testCase.limit, testCase.seconds, testCase.cancel)
			if output != (commandOutput{}) {
				t.Errorf("output = %#v, want zero output", output)
			}
			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("error = %v, want Isla error", err)
			}
			if failure.Code != testCase.code || failure.Detail != testCase.detail {
				t.Errorf("error = %q %q, want %q %q", failure.Code, failure.Detail, testCase.code, testCase.detail)
			}
		})
	}
}

func runSemanticResourceCase(t *testing.T, mode string, limit, seconds uint64, cancel bool) (commandOutput, error) {
	t.Helper()
	directory := t.TempDir()
	toolPath := filepath.Join(directory, "semantic-tool.sh")
	if err := os.WriteFile(toolPath, []byte(semanticResourceTool), 0o700); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	programPath := filepath.Join(directory, "program.toml")
	if err := os.WriteFile(programPath, []byte(mode), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	request := VerificationRequest{query: Request{program: Artifact{path: programPath}, timeLimit: seconds, maximumOutputSize: limit}}
	ctx := t.Context()
	if cancel {
		var stop context.CancelFunc
		ctx, stop = context.WithCancel(ctx)
		stop()
	}
	return (SemanticEngine{identity: ToolIdentity{Path: toolPath}}).runSemanticDump(ctx, request)
}
