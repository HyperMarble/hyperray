//go:build isla_integration

// Compiler evidence distinguishes retained constructs from optimized examples.
// Native results are test oracles, never proof-engine input.
package isla_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealRustPatternCompilerEvidence(t *testing.T) {
	cases := []struct {
		source    string
		fragments []string
	}{
		{"generic_calls.rs", []string{"\tcall\t", "\taddiw\t", "\taddi\t"}},
		{"dynamic_call.rs", []string{"\tjalr\t", "\t.quad\t", "\tld\t"}},
		{"recursive_calls.rs", []string{"\tcall\t", "\ttail\t", "\tsd\t", "\tld\t"}},
		{"stack_array.rs", []string{"\tsd\t", "\tld\t", "\tslli\t"}},
		{"stack_values.rs", []string{"\tsd\t", "\tld\t", "\tslli\t"}},
	}
	for _, example := range cases {
		t.Run(example.source, func(t *testing.T) {
			assembly := rustPatternCompilerEvidence(t, example.source)
			assertRustAssemblyFragments(t, assembly, example.fragments)
		})
	}
}

func rustPatternCompilerEvidence(t *testing.T, source string) string {
	t.Helper()
	compiler := requiredPath(t, "HYPERRAY_RUSTC")
	fixture := filepath.Join("../../fixtures/rust/machine", source)
	directory := t.TempDir()
	native := filepath.Join(directory, "native-test")
	assembly := filepath.Join(directory, "program.s")
	runRustBuildTool(t, compiler, "--edition=2021", "--test", fixture, "-o", native)
	runRustBuildTool(t, native)
	runRustBuildTool(t, compiler, "--edition=2021", "--crate-type=lib",
		"--target=riscv64gc-unknown-linux-gnu", "--emit=asm", "-C", "opt-level=2",
		"-C", "panic=abort", "-C", "relocation-model=static", "-C", "code-model=medium",
		fixture, "-o", assembly)
	content, err := os.ReadFile(assembly)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertRustAssemblyFragments(t *testing.T, assembly string, fragments []string) {
	t.Helper()
	for _, fragment := range fragments {
		count := strings.Count(assembly, fragment)
		if count == 0 {
			t.Errorf("compiler removed required construct %q\n%s", fragment, assembly)
		}
		t.Logf("compiler fragment %q: %d occurrences", fragment, count)
	}
}
