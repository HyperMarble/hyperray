// This check compares hardcoded Sail symbol names against the architecture.
// It must read the real architecture file, never a copy of the name list.
//go:build isla_integration && arm64_acceptance

package isla_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestRequiredSymbolsExistInArchitecture fails when native code names a Sail
// symbol the architecture does not define. Isla also rejects such a name, but
// only after loading the architecture, and its report names no symbol. This
// check names the symbol in under a tenth of a second.
func TestRequiredSymbolsExistInArchitecture(t *testing.T) {
	architecture := architectureText(t)
	missing := []string{}
	for _, symbol := range requiredSymbols(t) {
		if !strings.Contains(architecture, symbol) {
			missing = append(missing, symbol)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("architecture defines no such symbols: %s", strings.Join(missing, ", "))
	}
}

func architectureText(t *testing.T) string {
	t.Helper()
	path := os.Getenv("HYPERRAY_ARM64_SAIL_IR")
	if path == "" {
		t.Skip("HYPERRAY_ARM64_SAIL_IR is empty, so symbols cannot be compared")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read architecture: %v", err)
	}
	return string(content)
}

// requiredSymbols reads the names the ARM memory policy resolves at startup.
func requiredSymbols(t *testing.T) []string {
	t.Helper()
	root := os.Getenv(islaSourceRoot)
	if root == "" {
		t.Skipf("%s is empty, so symbols cannot be compared", islaSourceRoot)
	}
	content, err := os.ReadFile(filepath.Join(root, "isla-lib", "src", "arm_memory.rs"))
	if err != nil {
		t.Fatalf("read ARM memory policy: %v", err)
	}
	pattern := regexp.MustCompile(`"(z[A-Za-z0-9_]+)"`)
	seen := map[string]struct{}{}
	for _, match := range pattern.FindAllStringSubmatch(string(content), -1) {
		seen[match[1]] = struct{}{}
	}
	if len(seen) == 0 {
		t.Fatal("ARM memory policy named no Sail symbols")
	}
	symbols := make([]string, 0, len(seen))
	for symbol := range seen {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols
}
