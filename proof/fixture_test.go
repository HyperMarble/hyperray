// Test fixtures mint proof inputs through the public coverage API.
// They never construct a validated coverage capability directly.
package proof_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/HyperMarble/hyperray/coverage"
	"github.com/HyperMarble/hyperray/model"
)

type fixtureRequest struct {
	Model       model.Model                `json:"model"`
	Inventory   coverage.CompilerInventory `json:"inventory"`
	Certificate coverage.Certificate       `json:"certificate"`
}

func loadFixture(t *testing.T) fixtureRequest {
	t.Helper()
	content, err := os.ReadFile("../coverage/fixtures/complete.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	var request fixtureRequest
	if err := json.Unmarshal(content, &request); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return request
}

func requireCoverage(t *testing.T, request fixtureRequest) coverage.ValidatedCoverage {
	t.Helper()
	validated, err := coverage.Check(request.Model, request.Inventory, request.Certificate)
	if err != nil {
		t.Fatalf("coverage.Check() error = %v", err)
	}
	return validated
}

func validatedFixture(t *testing.T) coverage.ValidatedCoverage {
	t.Helper()
	return requireCoverage(t, loadFixture(t))
}

func modifiedFixture(t *testing.T, modify func(*fixtureRequest)) coverage.ValidatedCoverage {
	t.Helper()
	request := loadFixture(t)
	modify(&request)
	return requireCoverage(t, request)
}
