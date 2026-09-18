// A command that only describes the file must be accepted. A command that
// rewrites addresses at load time must not be, because the bytes it changes
// are not the bytes a proof would read.
package arm64_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/HyperMarble/hyperray/machine/arm64"
)

func loadWithFirstCommand(t *testing.T, id uint32) error {
	t.Helper()
	content := fixtureContent(t)
	binary.LittleEndian.PutUint32(content[32:36], id)
	_, err := arm64.LoadFunction(content, 32768,
		arm64.FunctionBoundary{StartAddress: 0x1000002e8, EndAddress: 0x1000002f0})
	return err
}

func TestADescribingCommandIsNotRejectedAsUnsupported(t *testing.T) {
	describing := map[string]uint32{
		"LC_CODE_SIGNATURE":  0x1d,
		"LC_FUNCTION_STARTS": 0x26,
		"LC_DATA_IN_CODE":    0x29,
		"LC_BUILD_VERSION":   0x32,
	}
	for name, id := range describing {
		var rejection *arm64.Rejection
		if errors.As(loadWithFirstCommand(t, id), &rejection) &&
			rejection.Code == arm64.UnsupportedLoadCommand {
			t.Errorf("%s (0x%x) was rejected as unsupported", name, id)
		}
	}
}

func TestACommandThatRewritesMemoryIsNotRejectedByItself(t *testing.T) {
	// The command states that a loader fills in addresses. Whether that
	// matters is decided per function by ReadsRewrittenAddress, so the
	// command alone is not a reason to refuse the file.
	rewriting := map[string]uint32{
		"LC_DYLD_INFO_ONLY": 0x80000022,
		"LC_DYSYMTAB":       0x0b,
		"LC_LOAD_DYLIB":     0x0c,
	}
	for name, id := range rewriting {
		var rejection *arm64.Rejection
		if errors.As(loadWithFirstCommand(t, id), &rejection) &&
			rejection.Code == arm64.UnsupportedLoadCommand {
			t.Errorf("%s (0x%x) was rejected as unsupported", name, id)
		}
	}
}
