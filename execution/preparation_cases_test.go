//go:build preparation_integration && darwin

// Cases change subjects, function paths, optimization, and declared input intervals.
// Fixture identity must never select production logic.
package execution_test

import (
	"fmt"
	"math"
	"path/filepath"
)

type preparationCase struct {
	name         string
	request      preparationRequest
	observations uint64
	broken       bool
	partial      bool
}

func preparationCases(root, directory string, tools map[string]string) []preparationCase {
	var cases []preparationCase
	for _, name := range []string{"stack_array", "mixed_width", "recursive_calls", "dynamic_call", "generic_calls", "combined"} {
		cases = append(cases, originalPreparations(root, directory, tools, name)...)
	}
	for _, interval := range [][2]uint64{{13, 23}, {23, 23}, {math.MaxUint64 - 3, math.MaxUint64}} {
		name := fmt.Sprintf("range-%d-%d", interval[0], interval[1])
		request := nativePreparation(root, filepath.Join(directory, name), tools)
		request.Limits.Minimum = interval[0]
		request.Limits.Maximum = interval[1]
		cases = append(cases, preparationCase{name, request, interval[1] - interval[0] + 1, false, false})
	}
	broken := nativePreparation(root, filepath.Join(directory, "broken"), tools)
	broken.Subject.Function = "workflow::broken"
	cases = append(cases, preparationCase{"broken", broken, 0, true, false})
	full := nativePreparation(root, filepath.Join(directory, "full-width"), tools)
	full.Limits.Minimum = 0
	full.Limits.Maximum = math.MaxUint64
	full.Limits.SearchDepth = 2
	return append(cases, preparationCase{"full-width-partial", full, 0, false, true})
}

func originalPreparations(root, directory string, tools map[string]string, name string) []preparationCase {
	var cases []preparationCase
	for _, optimization := range []uint8{0, 2} {
		label := fmt.Sprintf("%s-o%d", name, optimization)
		request := nativePreparation(root, filepath.Join(directory, label), tools)
		request.Optimization = optimization
		request.Subject = preparationFunction{filepath.Join(root, "fixtures", "rust", "machine", name+".rs"), "_start"}
		if name == "combined" {
			request.Subject.Source = filepath.Join(root, "..", "hyperray-research", "native-exhaustive", "combined.rs")
		}
		request.Requirement.Function = name
		request.Limits.Minimum = 0
		request.Limits.Maximum = 65535
		cases = append(cases, preparationCase{label, request, 65536, false, false})
	}
	return cases
}
