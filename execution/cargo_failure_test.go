//go:build preparation_integration && darwin

// Real Cargo errors must retain diagnostics and never publish prepared results.
// Invalid features and signatures must not fall back to another build path.
package execution_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCargoPreparationFailures(t *testing.T) {
	root, preparer, tools := preparationTools(t)
	cases := []struct {
		name, stage string
		change      func(*preparationRequest)
	}{
		{"feature", "cargo-add-subject", func(r *preparationRequest) {
			r.Cargo.Subject.Features = []string{"missing-feature"}
		}},
		{"signature", "cargo-build", func(r *preparationRequest) {
			r.Requirement.Function = "bad_signature"
		}},
		{"workspace", "cargo-add-subject", func(r *preparationRequest) {
			r.Subject.Source = filepath.Join(root, "fixtures", "rust", "cargo-preparation", "Cargo.toml")
		}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			request := cargoPreparation(t, root, filepath.Join(t.TempDir(), "checker"), tools)
			item.change(&request)
			output, err := invokePreparation(t, preparer, request)
			if err == nil || !strings.Contains(string(output), item.stage) {
				t.Fatalf("error = %v, output = %s", err, output)
			}
			cargoFailureEvidence(t, request.Directory, item.stage)
		})
	}
}

func cargoFailureEvidence(t *testing.T, directory, stage string) {
	t.Helper()
	log, err := os.ReadFile(filepath.Join(directory, stage+".log"))
	if err != nil || len(log) == 0 {
		t.Fatalf("missing compiler diagnostics: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "prepared.json")); !os.IsNotExist(err) {
		t.Fatalf("failed build published a result: %v", err)
	}
}
