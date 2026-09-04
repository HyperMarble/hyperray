// Trace-event scan tests distinguish direct events from nested values.
package isla

import (
	"reflect"
	"testing"
)

func TestScanTraceOutputRecordsDirectEvents(t *testing.T) {
	output := `(trace
	(   define-const v0 (bvadd #x01 #x02))
	(assume true) ; source(path)
	(read-reg |x2| nil #x00)
)
(trace
  (write-reg |x1| nil "value ( text"))`
	result, err := scanTraceOutput(output)
	if err != nil {
		t.Fatalf("scanTraceOutput() error = %v", err)
	}
	want := traceScan{
		TraceCount: 2,
		Events: []scannedTraceEvent{
			{TraceIndex: 0, EventIndex: 0, Kind: "define-const"},
			{TraceIndex: 0, EventIndex: 1, Kind: "assume"},
			{TraceIndex: 0, EventIndex: 2, Kind: "read-reg"},
			{TraceIndex: 1, EventIndex: 0, Kind: "write-reg"},
		},
	}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("scanTraceOutput() = %#v, want %#v", result, want)
	}
}

func TestTraceEventHeadRejectsQuotedKind(t *testing.T) {
	kind, err := traceEventHead("(|event| value)")
	if err == nil {
		t.Errorf("traceEventHead() = %q, error = nil", kind)
	}
}
