// Event ordering tests reject caller order and mutation as hidden inputs.
package isla_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestTraceEventCatalogIgnoresCallerOrder(t *testing.T) {
	instructions, report := coverageFixture(t)
	first, err := isla.InventoryTraceEvents(instructions, report)
	if err != nil {
		t.Fatalf("first InventoryTraceEvents() error = %v", err)
	}
	slices.Reverse(instructions)
	slices.Reverse(report.Instructions)
	second, err := isla.InventoryTraceEvents(instructions, report)
	if err != nil {
		t.Fatalf("second InventoryTraceEvents() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("ordered catalog changed: first=%#v second=%#v", first, second)
	}
}

func TestTraceEventCatalogOwnsItsValues(t *testing.T) {
	instructions, report := coverageFixture(t)
	catalog, err := isla.InventoryTraceEvents(instructions, report)
	if err != nil {
		t.Fatalf("InventoryTraceEvents() error = %v", err)
	}
	first := catalog.Events[0]
	instructions[0].Bytes[0] = 0xff
	report.Instructions[0].TraceOutput = "changed"
	if catalog.Events[0] != first {
		t.Errorf("event changed after caller mutation: %#v", catalog.Events[0])
	}
}
