// These tests prove that the public graph types retain exact JSON data.
// They never normalize names or state values.
package model_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/model"
)

func TestStateValuesRoundTrip(t *testing.T) {
	want := model.Model{
		States: []model.State{{
			ID: "ready",
			Values: map[string]string{
				"counter": "0007",
				"message": "exact value",
			},
		}},
		Transitions: []model.Transition{{
			ID:          "stay",
			FromStateID: "ready",
			ToStateID:   "ready",
		}},
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	wantJSON := `{"states":[{"id":"ready","values":{"counter":"0007","message":"exact value"}}],"transitions":[{"id":"stay","from_state_id":"ready","to_state_id":"ready"}]}`
	if string(encoded) != wantJSON {
		t.Errorf("json.Marshal() = %s, want %s", encoded, wantJSON)
	}
	var got model.Model
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON round trip = %#v, want %#v", got, want)
	}
}
