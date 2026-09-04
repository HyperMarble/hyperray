// Limited-buffer tests exercise both truncation boundaries directly.
package isla

import "testing"

func TestLimitedBufferConsumesExcessWrites(t *testing.T) {
	buffer := newLimitedBuffer(3)
	written, err := buffer.Write([]byte("abcd"))
	if err != nil || written != 4 {
		t.Fatalf("first Write() = %d, %v", written, err)
	}
	written, err = buffer.Write([]byte("ef"))
	if err != nil || written != 2 {
		t.Fatalf("second Write() = %d, %v", written, err)
	}
	if buffer.String() != "abc" || !buffer.exceeded || buffer.remaining() != 0 {
		t.Errorf("buffer = %q, exceeded = %t", buffer.String(), buffer.exceeded)
	}
}
