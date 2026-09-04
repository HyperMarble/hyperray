// Public event-catalog tests require exact source-order event identities.
package isla_test

import (
	"reflect"
	"testing"

	"github.com/HyperMarble/hyperray/machine/isla"
)

func TestPublicTraceEventCatalog(t *testing.T) {
	instructions, report := coverageFixture(t)
	catalog, err := isla.InventoryTraceEvents(instructions, report)
	if err != nil {
		t.Fatalf("InventoryTraceEvents() error = %v", err)
	}
	want := []isla.TraceEvent{
		{
			Identifier: "0000000000001000:0:0", Address: 0x1000,
			TraceIndex: 0, EventIndex: 0, Kind: "bytes",
			TraceDigest: report.Instructions[0].OutputDigest,
		},
		{
			Identifier: "0000000000001000:0:1", Address: 0x1000,
			TraceIndex: 0, EventIndex: 1, Kind: "initial",
			TraceDigest: report.Instructions[0].OutputDigest,
		},
		{
			Identifier: "0000000000001002:0:0", Address: 0x1002,
			TraceIndex: 0, EventIndex: 0, Kind: "bytes",
			TraceDigest: report.Instructions[1].OutputDigest,
		},
		{
			Identifier: "0000000000001002:0:1", Address: 0x1002,
			TraceIndex: 0, EventIndex: 1, Kind: "initial",
			TraceDigest: report.Instructions[1].OutputDigest,
		},
	}
	if !catalog.Complete || catalog.InstructionCount != 2 || catalog.TraceCount != 2 || catalog.EventCount != 4 {
		t.Errorf("catalog summary = %#v", catalog)
	}
	if !reflect.DeepEqual(catalog.Events, want) {
		t.Errorf("events = %#v, want %#v", catalog.Events, want)
	}
}
