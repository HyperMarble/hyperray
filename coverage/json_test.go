// JSON tests keep the canonical fixture equal to the public Go representation.
// They never normalize or omit evidence during a round trip.
package coverage_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestJSONFixtureRoundTrip(t *testing.T) {
	want, err := os.ReadFile("fixtures/complete.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	request := completeRequest(t)
	got, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Error("canonical JSON changed after its public round trip")
	}
}
