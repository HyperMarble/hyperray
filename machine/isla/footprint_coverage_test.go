// Public coverage tests require exact bidirectional inventory equality.
package isla_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicFootprintCoverage(t *testing.T) {
	instructions, report := coverageFixture(t)
	coverage, err := isla.ValidateFootprintInventory(instructions, report)
	if err != nil {
		t.Fatalf("ValidateFootprintInventory() error = %v", err)
	}
	if !coverage.Complete || coverage.CoveredInstructions != 2 || coverage.TotalInstructions != 2 {
		t.Errorf("coverage = %#v", coverage)
	}
}

func TestFootprintCoverageRejectsInvalidSource(t *testing.T) {
	_, report := coverageFixture(t)
	coverage, err := isla.ValidateFootprintInventory(nil, report)
	if err == nil {
		t.Fatalf("coverage = %#v, error = nil", coverage)
	}
}
