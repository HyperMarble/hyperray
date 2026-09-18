// This external test covers zero-fill placement in a mutated Mach-O fixture.
// It must keep file-backed bytes separate from zero-fill sections.
package arm64_test

import (
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func TestLoadFunctionAcceptsZeroFillInsideFileBackedRange(t *testing.T) {
	content := mutatedLinkeditWithZeroFill(t, 0x100004010)
	image, err := arm64.LoadFunctionPages(content, 32768, mutatedTextBoundary(t, content), zeroFillPages(0x100004010))
	if err != nil {
		t.Fatalf("LoadFunction() error = %v", err)
	}
	assertInsertedZeroFill(t, content, image, 0x100004010)
}

func TestLoadFunctionAcceptsZeroFillTail(t *testing.T) {
	content := mutatedLinkeditWithZeroFill(t, 0x100004048)
	image, err := arm64.LoadFunctionPages(content, 32768, mutatedTextBoundary(t, content), zeroFillPages(0x100004048))
	if err != nil {
		t.Fatalf("LoadFunction() error = %v", err)
	}
	assertInsertedZeroFill(t, content, image, 0x100004048)
}

func TestLoadFunctionAcceptsGreaterZeroFillTail(t *testing.T) {
	content := mutatedLinkeditWithGreaterZeroFill(t, 0x100004048)
	image, err := arm64.LoadFunctionPages(content, 32768, mutatedTextBoundary(t, content), zeroFillPages(0x100004048))
	if err != nil {
		t.Fatalf("LoadFunction() error = %v", err)
	}
	assertInsertedZeroFill(t, content, image, 0x100004048)
}

func TestLoadFunctionRejectsZeroFillBeyondSegment(t *testing.T) {
	content := mutatedLinkeditWithZeroFill(t, 0x100008004)
	image, err := arm64.LoadFunction(content, 32768, mutatedTextBoundary(t, content))
	assertLoadFailure(t, image, err, arm64.InvalidSection)
}

func mutatedTextBoundary(t *testing.T, content []byte) arm64.FunctionBoundary {
	t.Helper()
	file := openMachoContent(t, content)
	text := arm64Text(t, file)
	return arm64.FunctionBoundary{StartAddress: text.Addr, EndAddress: text.Addr + text.Size}
}

// zeroFillPages returns the pages of a zero-fill section, as the loader
// would place them when a proof reaches that address.
func zeroFillPages(address uint64) []uint64 {
	return []uint64{address / arm64.PageSize}
}
