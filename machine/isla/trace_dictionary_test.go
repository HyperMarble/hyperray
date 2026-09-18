// A dictionary must restore the identical text, and must report a malformed
// input rather than return partial text.
package isla

import (
	"strings"
	"testing"
)

func TestADictionaryRestoresTheSameText(t *testing.T) {
	text := "(read-reg |A| nil 1)\n(write-reg |B| nil 2)\n(read-reg |A| nil 1)"
	restored, err := dictionaryDecode(dictionaryEncode(text))
	if err != nil {
		t.Fatalf("dictionaryDecode() error = %v", err)
	}
	if restored != text {
		t.Fatalf("dictionaryDecode() = %q, want %q", restored, text)
	}
}

func TestARepeatedLineIsNamedOnce(t *testing.T) {
	line := "(read-reg |A| nil 1)"
	text := strings.Repeat(line+"\n", 100)
	encoded := dictionaryEncode(text)
	if strings.Count(encoded, line) != 1 {
		t.Fatalf("line appears %d times, want 1", strings.Count(encoded, line))
	}
	if len(encoded) >= len(text) {
		t.Fatalf("encoded %d bytes is not smaller than %d", len(encoded), len(text))
	}
}

func TestTextRemainsReadable(t *testing.T) {
	text := "(read-reg |GTEExtObs| nil 1)\n(write-reg |B| nil 2)"
	encoded := dictionaryEncode(text)
	if !strings.Contains(encoded, "GTEExtObs") {
		t.Fatal("encoded form does not contain the original text")
	}
}

func TestAMissingOrderSectionIsReported(t *testing.T) {
	if _, err := dictionaryDecode("just one line"); err == nil {
		t.Fatal("dictionaryDecode() accepted text with no order section")
	}
}

func TestAnOrderNamingNoLineIsReported(t *testing.T) {
	if _, err := dictionaryDecode("one\ntwo\n\n0,9"); err == nil {
		t.Fatal("dictionaryDecode() accepted an out-of-range order")
	}
}
