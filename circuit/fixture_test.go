// Fixture support builds public relations from all balance fixture records.
// It never selects behavior from a fixture file name.
package circuit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type balanceFixture struct {
	Width               uint16 `json:"width"`
	ReferenceComparison string `json:"reference_comparison"`
	CandidateComparison string `json:"candidate_comparison"`
	ExpectedStatus      string `json:"expected_status"`
	DifferentPairs      uint64 `json:"different_pairs"`
	ExpectedBalance     string `json:"expected_balance"`
	ExpectedCost        string `json:"expected_cost"`
	ExpectedReference   string `json:"expected_reference"`
	ExpectedCandidate   string `json:"expected_candidate"`
}

func loadBalanceFixtures(t *testing.T) []balanceFixture {
	t.Helper()
	paths, err := filepath.Glob("../fixtures/circuit/*.json")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no circuit fixtures")
	}
	fixtures := make([]balanceFixture, 0, len(paths))
	for _, path := range paths {
		content, readError := os.ReadFile(path)
		if readError != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", path, readError)
		}
		var fixture balanceFixture
		if decodeError := json.Unmarshal(content, &fixture); decodeError != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", path, decodeError)
		}
		fixtures = append(fixtures, fixture)
	}
	return fixtures
}

func fixtureWithStatus(t *testing.T, status string) balanceFixture {
	t.Helper()
	matches := make([]balanceFixture, 0, 1)
	for _, fixture := range loadBalanceFixtures(t) {
		if fixture.ExpectedStatus == status {
			matches = append(matches, fixture)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("fixtures with status %q = %d, want 1", status, len(matches))
	}
	return matches[0]
}
