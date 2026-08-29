// Tests for the language-aware raise breakers: each language derives the
// wrong-type and no-raise edits from its own raise syntax, and a site that
// cannot be derived returns "" instead of a wrong edit.
package tests

import (
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/enforce"
)

const pythonRaiseSource = `def check(parts):
    if not parts:
        raise ValueError("at least one component")
    return parts
`

const rustPanicSource = `fn check(parts: &[i32]) {
    if parts.is_empty() {
        panic!("at least one component");
    }
}
`

const cppThrowSource = `void check(int n) {
    if (n == 0) {
        throw std::invalid_argument("at least one component");
    }
}
`

func TestBreakers_PythonSwapAndSuppress(t *testing.T) {
	swapped := enforce.SwapRaiseType("python", pythonRaiseSource, "ValueError", "at least one component")
	if !strings.Contains(swapped, `raise RuntimeError("at least one component")`) {
		t.Fatalf("swap failed: %s", swapped)
	}
	suppressed := enforce.SuppressRaise("python", pythonRaiseSource, "ValueError", "at least one component")
	if strings.Contains(suppressed, "raise ValueError") || !strings.Contains(suppressed, "pass") {
		t.Fatalf("suppress failed: %s", suppressed)
	}
}

func TestBreakers_RustSuppressesPanicAndSkipsSwap(t *testing.T) {
	if got := enforce.SwapRaiseType("rust", rustPanicSource, "ValueError", "at least one component"); got != "" {
		t.Fatalf("rust has no type swap; got: %s", got)
	}
	suppressed := enforce.SuppressRaise("rust", rustPanicSource, "ValueError", "at least one component")
	if strings.Contains(suppressed, "panic!") || !strings.Contains(suppressed, "()") {
		t.Fatalf("suppress failed: %s", suppressed)
	}
}

func TestBreakers_CppSwapAndSuppress(t *testing.T) {
	swapped := enforce.SwapRaiseType("cpp", cppThrowSource, "std::invalid_argument", "at least one component")
	if !strings.Contains(swapped, `throw std::logic_error("at least one component")`) {
		t.Fatalf("swap failed: %s", swapped)
	}
	suppressed := enforce.SuppressRaise("cpp", cppThrowSource, "std::invalid_argument", "at least one component")
	if strings.Contains(suppressed, "throw") {
		t.Fatalf("suppress failed: %s", suppressed)
	}
}

func TestBreakers_UnderivableSiteReturnsEmpty(t *testing.T) {
	if got := enforce.SwapRaiseType("python", "x = 1\n", "ValueError", "missing message"); got != "" {
		t.Fatalf("expected empty for underivable site, got: %s", got)
	}
	if got := enforce.SuppressRaise("cpp", "int x;\n", "std::invalid_argument", "missing"); got != "" {
		t.Fatalf("expected empty for underivable site, got: %s", got)
	}
}
