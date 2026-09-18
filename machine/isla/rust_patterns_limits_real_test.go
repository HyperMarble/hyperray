//go:build isla_integration

// Compiler stress cases declare resource limits distinct from proof boundaries.
// Exhausted resources must return an error, never a proof result.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

const rustPatternTimeLimitSeconds = 120
const rustPatternOutputLimitBytes = 64 * 1024 * 1024
const rustPatternMemoryLimitMB = 2048
const rustPatternThreadLimit = 2

// The recursive fixture permits three recursive steps and the depth-zero call.
const rustPatternPCVisitLimit = 4

func rustPatternRequest(t *testing.T, path string) isla.VerificationRequest {
	t.Helper()
	return rustPatternVisitRequest(t, path, rustPatternPCVisitLimit)
}

func rustPatternVisitRequest(t *testing.T, path string, visits uint64) isla.VerificationRequest {
	t.Helper()
	architecture := realArtifact(t, "HYPERRAY_SAIL_IR")
	configuration := realArtifact(t, "HYPERRAY_ISLA_CONFIG")
	memoryModel := realArtifact(t, "HYPERRAY_MEMORY_MODEL")
	program := identifiedArtifact(t, path)
	query, err := isla.NewRequest(architecture, configuration, memoryModel, program,
		visits, rustPatternTimeLimitSeconds, rustPatternOutputLimitBytes)
	if err != nil {
		t.Fatal(err)
	}
	request, err := isla.NewVerificationRequest(query, rustPatternThreadLimit, rustPatternMemoryLimitMB)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
