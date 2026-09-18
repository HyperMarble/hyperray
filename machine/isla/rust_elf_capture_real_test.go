//go:build isla_integration

package isla_test

import (
	"crypto/sha256"
	"os"
	"strings"
	"testing"
)

func captureRustELF(t *testing.T, content []byte) {
	t.Helper()
	captureDir := os.Getenv("HYPERRAY_RUST_ELF_CAPTURE_DIR")
	if captureDir == "" {
		return
	}
	if err := os.MkdirAll(captureDir, 0o700); err != nil {
		t.Fatal(err)
	}
	prefix := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name()) + "-"
	captured, err := os.CreateTemp(captureDir, prefix+"*.elf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := captured.Write(content); err != nil {
		closeErr := captured.Close()
		if closeErr != nil {
			t.Fatalf("write captured ELF: %v; close: %v", err, closeErr)
		}
		t.Fatal(err)
	}
	if err := captured.Close(); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	t.Logf("captured linked ELF: %s sha256=%x", captured.Name(), digest)
}
