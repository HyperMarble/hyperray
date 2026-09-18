// Both program stages require the same explicit memory capability.
// A legacy query must not select sequential memory implicitly.
package isla

import (
	"slices"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func TestSequentialMemoryCapabilityArguments(t *testing.T) {
	query := Request{executableProgram: true, sequentialMemory: true, configuration: Artifact{path: "model.toml"}}
	request := VerificationRequest{query: query}
	expected := []string{"--sequential-memory", "-I", "__monomorphize_reads = true", "--footprint-config", "model.toml"}
	for _, arguments := range [][]string{query.arguments(), request.semanticArguments()} {
		if len(arguments) < len(expected) {
			t.Errorf("missing sequential arguments: %v", arguments)
			continue
		}
		if !slices.Equal(arguments[len(arguments)-len(expected):], expected) {
			t.Errorf("sequential program options differ: %v", arguments)
		}
	}
}

func TestAxiomaticMemoryKeepsAddressPartitionDisabled(t *testing.T) {
	query := Request{executableProgram: true}
	request := VerificationRequest{query: query}
	for _, arguments := range [][]string{query.arguments(), request.semanticArguments()} {
		if slices.Contains(arguments, "--sequential-memory") {
			t.Errorf("axiomatic program selects sequential memory: %v", arguments)
		}
		if slices.Contains(arguments, "-I") || slices.Contains(arguments, "--footprint-config") {
			t.Errorf("axiomatic program overrides model controls: %v", arguments)
		}
	}
}

func TestFootprintKeepsSymbolicMemoryAddresses(t *testing.T) {
	request := FootprintRequest{}
	arguments := request.arguments(machine.Instruction{})
	if slices.Contains(arguments, "__monomorphize_reads = true") {
		t.Errorf("isolated instruction enumerates memory addresses: %v", arguments)
	}
	if slices.Contains(arguments, "--footprint-config") {
		t.Errorf("isolated instruction overrides its model configuration: %v", arguments)
	}
}
