package tests

import (
	"os"
	"strings"
	"testing"

	"github.com/HyperMarble/ray/internal/sufficiency"
)

func writeTempFile(t *testing.T, pattern, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// TestSufficiency_MultiLanguage locks in the reason extract_outcomes.py
// uses tree-sitter rather than a hand-rolled walk of one language's AST:
// the same extractor, the same code path, and each language's own real
// maintained grammar covers all four of ray's targets. An earlier
// Python-`ast`-based version could never have worked for Rust, C++, or
// Go at all.
func TestSufficiency_MultiLanguage(t *testing.T) {
	python := testPython3(t)
	script := extractScriptPath(t)

	t.Run("rust_panic_and_return", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.rs", `
fn f(n: i32) -> i32 {
    if n == 0 { panic!("at least one component"); }
    if n < 0 { return 0; }
    println!("not an outcome");
    return n + 1;
}
`)
		outcomes, err := sufficiency.ExtractOutcomes(python, script, src, sufficiency.LangRust)
		if err != nil {
			t.Fatalf("ExtractOutcomes: %v", err)
		}
		// println! is a macro too, but not a terminating one -- it must not
		// be mistaken for an outcome.
		if len(outcomes) != 3 {
			t.Fatalf("got %d outcomes, want 3 (println! must not count): %+v", len(outcomes), outcomes)
		}
		if outcomes[0].Kind != "raise" ||
			!strings.Contains(outcomes[0].SourceText, "panic!") ||
			!strings.Contains(outcomes[0].SourceText, "at least one component") {
			t.Errorf("outcome 0 = %+v, want the verbatim panic! text", outcomes[0])
		}
	})

	t.Run("cpp_throw_and_return", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.cpp", `
int f(int n) {
    if (n == 0) { throw std::invalid_argument("at least one component"); }
    if (n < 0) { return 0; }
    return n + 1;
}
`)
		outcomes, err := sufficiency.ExtractOutcomes(python, script, src, sufficiency.LangCPP)
		if err != nil {
			t.Fatalf("ExtractOutcomes: %v", err)
		}
		if len(outcomes) != 3 {
			t.Fatalf("got %d outcomes, want 3: %+v", len(outcomes), outcomes)
		}
		// The verbatim text keeps the qualified name as written, so spec.md
		// can quote either "invalid_argument" or the message -- no
		// name-munging rule needed.
		if outcomes[0].Kind != "raise" ||
			!strings.Contains(outcomes[0].SourceText, "invalid_argument") ||
			!strings.Contains(outcomes[0].SourceText, "at least one component") {
			t.Errorf("outcome 0 = %+v, want the verbatim throw text", outcomes[0])
		}
	})

	t.Run("go_multivalue_return", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.go", `
package main

func f(n int) (int, error) {
	if n == 0 {
		return 0, errors.New("at least one component")
	}
	return n + 1, nil
}
`)
		outcomes, err := sufficiency.ExtractOutcomes(python, script, src, sufficiency.LangGo)
		if err != nil {
			t.Fatalf("ExtractOutcomes: %v", err)
		}
		if len(outcomes) != 2 {
			t.Fatalf("got %d outcomes, want 2: %+v", len(outcomes), outcomes)
		}
		// Go has no throw: a returned error is an ordinary return, and its
		// full multi-value source text is what spec.md would match against.
		if outcomes[0].Kind != "return" ||
			outcomes[0].SourceText != `return 0, errors.New("at least one component")` {
			t.Errorf("outcome 0 = %+v, want the full multi-value return text", outcomes[0])
		}
	})

	// The next two lock in gaps found by testing edge syntaxes rather
	// than by speculating that gaps might exist. Both were real: the
	// extractor silently missed a language's most common exit path.
	t.Run("rust_question_mark_is_an_exit", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.rs", `
fn f(a: i32) -> Result<i32, E> {
    let v = risky()?;
    if a == 0 { return Err(E); }
    Ok(v)
}
`)
		outcomes, err := sufficiency.ExtractOutcomes(python, script, src, sufficiency.LangRust)
		if err != nil {
			t.Fatalf("ExtractOutcomes: %v", err)
		}
		// `risky()?` returns Err early -- a Result-returning function's
		// most common error path. Missing it made that path invisible.
		found := false
		for _, o := range outcomes {
			if o.Kind == "raise" && strings.Contains(o.SourceText, "risky()?") {
				found = true
			}
		}
		if !found {
			t.Fatalf("the `?` early-exit was not extracted: %+v", outcomes)
		}
	})

	t.Run("go_naked_return_is_an_outcome", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.go", `
package main

func f(a int) (res int, err error) {
	if a == 0 {
		return
	}
	return a, nil
}
`)
		outcomes, err := sufficiency.ExtractOutcomes(python, script, src, sufficiency.LangGo)
		if err != nil {
			t.Fatalf("ExtractOutcomes: %v", err)
		}
		// Unlike Python/C++/Rust, a naked return in Go returns the
		// function's NAMED results -- a real outcome with real values, not
		// the "returns nothing" case excluded elsewhere.
		if len(outcomes) != 2 {
			t.Fatalf("got %d outcomes, want 2 (naked return counts in Go): %+v", len(outcomes), outcomes)
		}
	})

	// The most consequential gap a 160-file stress sweep found: all Verus
	// code lives inside `verus! { ... }`, and tree-sitter parses a macro
	// body as an opaque token_tree -- so ray extracted ZERO outcomes from
	// exactly the Rust source it cares about most. Fixed using
	// tree-sitter-rust's own shipped injections.scm declaration (re-parse
	// a macro body as Rust), not a verus-specific special case.
	t.Run("rust_verus_macro_body_is_parsed", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.rs", `
use vstd::prelude::*;
verus! {
fn clamp(x: i32, lo: i32) -> (r: i32)
    ensures r >= lo,
{
    if x < lo { return lo; }
    return x;
}
}
`)
		outcomes, err := sufficiency.ExtractOutcomes(python, script, src, sufficiency.LangRust)
		if err != nil {
			t.Fatalf("ExtractOutcomes: %v", err)
		}
		if len(outcomes) != 2 {
			t.Fatalf("got %d outcomes, want 2 from inside verus!{}: %+v", len(outcomes), outcomes)
		}
		// Line numbers must point at the real file, not at an offset into
		// the macro body.
		if outcomes[0].Line != 7 || outcomes[1].Line != 8 {
			t.Errorf("got lines %d,%d want 7,8 -- injection line offset is wrong",
				outcomes[0].Line, outcomes[1].Line)
		}
	})

	t.Run("unsupported_language_rejected", func(t *testing.T) {
		src := writeTempFile(t, "sufficiency-*.txt", "whatever\n")
		_, err := sufficiency.ExtractOutcomes(python, script, src, "cobol")
		if err == nil {
			t.Fatal("want an error for an unsupported language, got nil")
		}
	})
}
