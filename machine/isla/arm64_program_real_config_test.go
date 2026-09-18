//go:build isla_integration && arm64_acceptance

// This helper binds witness aliases to the selected ARM configuration.
// It must not rely on substring matching or an unrelated RISC-V config.
package isla_test

import (
	"bytes"
	"os"
	"testing"
)

func arm64R0Names(t *testing.T) []string {
	t.Helper()
	content, err := os.ReadFile(requiredPath(t, "HYPERRAY_ARM64_ISLA_CONFIG"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !bytes.Contains(content, []byte(`"X0" = "R0"`)) {
		t.Fatal("ARM configuration lacks the X0 to R0 rename")
	}
	return []string{"0:R0", "0:X0"}
}
