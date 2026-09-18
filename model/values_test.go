// These tests define the exact JSON rule for nil and empty state values.
// They never treat map allocation state as witness data.
package model_test

import (
	"encoding/json"
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestNilAndEmptyValuesHaveCanonicalJSON(t *testing.T) {
	nilValues := model.State{ID: "ready"}
	emptyValues := model.State{ID: "ready", Values: map[string]string{}}
	nilJSON, err := json.Marshal(nilValues)
	if err != nil {
		t.Fatalf("json.Marshal(nil Values) error = %v", err)
	}
	emptyJSON, err := json.Marshal(emptyValues)
	if err != nil {
		t.Fatalf("json.Marshal(empty Values) error = %v", err)
	}
	want := `{"id":"ready"}`
	if string(nilJSON) != want || string(emptyJSON) != want {
		t.Errorf("nil JSON = %s, empty JSON = %s, want %s", nilJSON, emptyJSON, want)
	}
	var decoded model.State
	if err := json.Unmarshal([]byte(`{"id":"ready","values":{}}`), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(empty Values) error = %v", err)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal(decoded state) error = %v", err)
	}
	if string(canonical) != want {
		t.Errorf("canonical JSON = %s, want %s", canonical, want)
	}
	for _, state := range []model.State{nilValues, emptyValues} {
		if err := model.Validate(model.Model{States: []model.State{state}}); err != nil {
			t.Errorf("Validate(%#v) error = %v", state.Values, err)
		}
	}
}
