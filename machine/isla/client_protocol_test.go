// Protocol tests drive the reader with bytes an isla-client would write.
// They must reject a truncated or unknown reply instead of returning traces.
package isla

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func traceReply(bodies ...string) []byte {
	out := []byte{answerStartTraces}
	for _, body := range bodies {
		out = append(out, answerTrace, 1)
		header := make([]byte, 4)
		binary.LittleEndian.PutUint32(header, uint32(len(body)))
		out = append(out, header...)
		out = append(out, body...)
	}
	return append(out, answerEndTraces)
}

func TestReadTracesReturnsEveryTrace(t *testing.T) {
	traces, err := ReadTraces(bytes.NewReader(traceReply("first", "second")))
	if err != nil {
		t.Fatalf("ReadTraces() error = %v", err)
	}
	if len(traces) != 2 {
		t.Fatalf("ReadTraces() returned %d traces, want 2", len(traces))
	}
	if string(traces[0]) != "first" || string(traces[1]) != "second" {
		t.Errorf("ReadTraces() returned %q and %q", traces[0], traces[1])
	}
}

func TestReadTracesRejectsAnError(t *testing.T) {
	reply := append(traceReply("kept")[:0:0], answerStartTraces, answerError)
	if _, err := ReadTraces(bytes.NewReader(reply)); err == nil {
		t.Fatal("ReadTraces() accepted an error answer")
	}
}

func TestReadTracesRejectsAnUnknownTag(t *testing.T) {
	if _, err := ReadTraces(bytes.NewReader([]byte{answerStartTraces, 9})); err == nil {
		t.Fatal("ReadTraces() accepted an unknown tag")
	}
}

func TestReadTracesRejectsATruncatedBody(t *testing.T) {
	reply := []byte{answerStartTraces, answerTrace, 1, 10, 0, 0, 0, 'a', 'b'}
	if _, err := ReadTraces(bytes.NewReader(reply)); err == nil {
		t.Fatal("ReadTraces() accepted a truncated body")
	}
}

func TestWriteMessageSendsLengthThenBody(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeMessage(&buffer, "version"); err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}
	written := buffer.Bytes()
	if binary.LittleEndian.Uint32(written[:4]) != 7 {
		t.Errorf("writeMessage() wrote length %d, want 7", binary.LittleEndian.Uint32(written[:4]))
	}
	if string(written[4:]) != "version" {
		t.Errorf("writeMessage() wrote body %q", written[4:])
	}
}
