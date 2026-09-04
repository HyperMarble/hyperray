// These external tests load real ELF fixtures through the public API.
// They must observe rejections without access to package internals.
package machine_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HyperMarble/hyperray/machine"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "fixtures", "machine", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return content
}

func acceptedFixture(t *testing.T, name string) (machine.Image, []byte) {
	t.Helper()
	content := fixture(t, name)
	image, err := machine.Load(content, uint64(len(content)))
	if err != nil {
		t.Fatalf("machine.Load(%q) error = %v", name, err)
	}
	return image, content
}

func requireRejection(t *testing.T, content []byte, maximum uint64, want machine.RejectionCode) *machine.Rejection {
	t.Helper()
	_, err := machine.Load(content, maximum)
	if err == nil {
		t.Fatalf("machine.Load() error = nil, want %q", want)
	}
	var rejection *machine.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("machine.Load() error = %T, want *machine.Rejection", err)
	}
	if rejection.Code != want {
		t.Fatalf("rejection code = %q, want %q: %v", rejection.Code, want, err)
	}
	if rejection.Detail == "" || !strings.Contains(err.Error(), machine.ProfileName) {
		t.Fatalf("rejection does not give an exact public cause: %v", err)
	}
	return rejection
}
