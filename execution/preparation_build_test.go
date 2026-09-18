//go:build preparation_integration && darwin

// Invoke the JSON preparer and preserve compiler diagnostics in test failures.
// A failed build cannot be interpreted as an observed execution.
package execution_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func prepareNative(t *testing.T, executable string, request preparationRequest) preparedResult {
	t.Helper()
	output, err := invokePreparation(t, executable, request)
	if err != nil {
		logs, listError := filepath.Glob(filepath.Join(request.Directory, "*.log"))
		if listError != nil {
			t.Fatal(listError)
		}
		for _, path := range logs {
			content, readError := os.ReadFile(path)
			t.Logf("%s: %s; read error: %v", path, content, readError)
		}
		t.Fatalf("prepare: %v\n%s", err, output)
	}
	var prepared preparedResult
	if err := json.Unmarshal(output, &prepared); err != nil {
		t.Fatal(err)
	}
	if prepared.Minimum != request.Limits.Minimum || prepared.Maximum != request.Limits.Maximum {
		t.Fatal("preparer changed the declared interval")
	}
	expectedTools := len(request.Tools)
	if request.Cargo != nil {
		expectedTools++
	}
	if len(prepared.Tools) != expectedTools {
		t.Fatal("missing tool versions")
	}
	for _, tool := range prepared.Tools {
		if tool.Version == "" {
			t.Fatal("empty tool version")
		}
	}
	return prepared
}

func invokePreparation(t *testing.T, executable string, request preparationRequest) ([]byte, error) {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable)
	command.Stdin = bytes.NewReader(encoded)
	return command.CombinedOutput()
}
