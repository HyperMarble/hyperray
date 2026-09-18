//go:build preparation_integration && darwin

// Cargo projects must use one preparation path across feature selections.
// A compiler success alone is not a successful bounded search.
package execution_test

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCargoPreparationMatrix(t *testing.T) {
	root, preparer, tools := preparationTools(t)
	fixture := filepath.Join(root, "fixtures", "rust", "cargo-preparation")
	before := cargoSnapshot(t, fixture)
	defer func() {
		if !reflect.DeepEqual(before, cargoSnapshot(t, fixture)) {
			t.Error("preparation changed the input project")
		}
	}()
	cases := []struct {
		name, requirement string
		features          cargoFeatureSelection
		broken            bool
	}{
		{"no-defaults", "five", cargoFeatureSelection{false, []string{}}, false},
		{"defaults", "nine", cargoFeatureSelection{true, []string{}}, false},
		{"explicit", "nine", cargoFeatureSelection{false, []string{"boost"}}, false},
		{"broken", "five", cargoFeatureSelection{false, []string{"broken"}}, true},
	}
	var maximum int64
	completed := 0
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			request := cargoPreparation(t, root, filepath.Join(t.TempDir(), "checker"), tools)
			request.Cargo.Subject = item.features
			request.Requirement.Function = item.requirement
			peak := observeCargoPreparation(t, preparer, request, item.broken)
			maximum = max(maximum, peak)
			completed++
		})
	}
	t.Logf("CARGO_MATRIX completed=%d declared=%d peak_bytes=%d", completed, len(cases), maximum)
}

func observeCargoPreparation(t *testing.T, preparer string, request preparationRequest, broken bool) int64 {
	t.Helper()
	prepared := prepareNative(t, preparer, request)
	cargoBuildEvidence(t, request.Directory)
	result := observePrepared(t, prepared, false, broken)
	item := preparationCase{broken: broken, observations: 11}
	return validatePreparationOutput(t, item, prepared, result)
}
