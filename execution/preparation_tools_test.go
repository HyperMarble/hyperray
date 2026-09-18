//go:build preparation_integration && darwin

// Build the current public preparer and resolve installed tools for real tests.
// Missing tools must fail this opt-in matrix rather than skip a fixture.
package execution_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func preparationTools(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tools := make(map[string]string)
	for _, name := range []string{"rustc", "spin", "clang"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		tools[name] = path
	}
	adapter := filepath.Join(root, "adapters", "rust")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "cargo", "build", "--bin", "hyperray-native-prepare")
	command.Dir = adapter
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build preparer: %v\n%s", err, output)
	}
	return root, filepath.Join(adapter, "target", "debug", "hyperray-native-prepare"), tools
}

func nativePreparation(root, directory string, tools map[string]string) preparationRequest {
	fixtures := filepath.Join(root, "fixtures", "rust", "preparation")
	return preparationRequest{
		Subject:     preparationFunction{filepath.Join(fixtures, "subject.rs"), "workflow::solve"},
		Requirement: preparationFunction{filepath.Join(fixtures, "requirements.rs"), "transformed"},
		Tools:       tools, Directory: directory, Optimization: 0,
		Limits: preparationLimits{13, 23, 300, 18, 32, 10000, 1000000, 100000000},
	}
}
