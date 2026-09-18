//go:build isla_integration

// The explicit sequential contract must retain compiler-generated memory effects.
// A mixed-width read must not disappear through whole-access constraints.
package isla_test

import "testing"

func TestRealRustSequentialMixedWidth(t *testing.T) {
	assembly := rustPatternCompilerEvidence(t, "mixed_width.rs")
	assertRustAssemblyFragments(t, assembly, []string{"\tsb\t", "\tlhu\t"})
	verifyRustPattern(t, rustMachineCase{
		name: "mixed_width", source: "mixed_width.rs", input: 0x2211, expected: 0x2211,
	})
}
