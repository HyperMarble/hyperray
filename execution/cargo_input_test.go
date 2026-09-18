//go:build preparation_integration && darwin

// Cargo requests use the existing public preparation boundary with explicit features.
// Missing options must not become guessed defaults.
package execution_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

type cargoFeatureSelection struct {
	DefaultFeatures bool     `json:"default_features"`
	Features        []string `json:"features"`
}

type cargoPreparationOptions struct {
	Executable  string                `json:"executable"`
	Subject     cargoFeatureSelection `json:"subject"`
	Requirement cargoFeatureSelection `json:"requirement"`
}

func cargoPreparation(t *testing.T, root, directory string, tools map[string]string) preparationRequest {
	t.Helper()
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Fatal(err)
	}
	request := nativePreparation(root, directory, tools)
	fixture := filepath.Join(root, "fixtures", "rust", "cargo-preparation")
	request.Subject = preparationFunction{filepath.Join(fixture, "subject", "Cargo.toml"), "solve"}
	request.Requirement = preparationFunction{filepath.Join(fixture, "requirement", "Cargo.toml"), "five"}
	request.Cargo = &cargoPreparationOptions{cargo, cargoFeatureSelection{false, []string{}}, cargoFeatureSelection{false, []string{}}}
	return request
}
