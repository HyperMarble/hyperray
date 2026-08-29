package tests

import (
	"testing"

	"github.com/HyperMarble/hyperray/internal/oracle"
)

// taintBoundedCases locks in a real finding: a real task
// (bandit-interprocedural-taint-checks, from the deep-swe corpus) looked
// like the "AST/call-graph-shaped, not provable" case the spec skill used
// to warn about -- and it was provable the whole time. Its real
// TaintTracker.is_tainted logic is recursion over a finite AST tree, not
// an unbounded call graph. Each case here is one real rule from that
// actual source, bounded to a small fixed-depth tagged-data encoding
// (never a real ast.AST object) and proved directly -- confirming the
// "Recursive or graph-shaped clauses" section of skills/spec/SKILL.md
// works, not just reads plausibly.
//
// Every falseClaim is the true claim's implication with its conclusion
// negated (not a standalone conjunction) -- so a REFUTED verdict comes
// from a real, meaningful counterexample to the actual rule, not from a
// universally-quantified claim that's merely vacuous over most of the
// domain.
var taintBoundedCases = []struct {
	name       string
	src        string
	requires   string
	trueClaim  string
	falseClaim string
}{
	{
		"binop_soundness",
		"def is_tainted(kind0: int, kind1: int, kind2: int):\n" +
			"    def leaf(k):\n        return k == 0\n" +
			"    left = leaf(kind0) or leaf(kind1)\n" +
			"    result = left or leaf(kind2)\n" +
			"    return result\n",
		"kind0 >= 0 and kind0 <= 1 and kind1 >= 0 and kind1 <= 1 and kind2 >= 0 and kind2 <= 1",
		"not (kind0 == 1 and kind1 == 1 and kind2 == 1) or result == False",
		"not (kind0 == 1 and kind1 == 1 and kind2 == 1) or result == True",
	},
	{
		"binop_completeness",
		"def is_tainted(kind0: int, kind1: int, kind2: int):\n" +
			"    def leaf(k):\n        return k == 0\n" +
			"    left = leaf(kind0) or leaf(kind1)\n" +
			"    result = left or leaf(kind2)\n" +
			"    return result\n",
		"kind0 >= 0 and kind0 <= 1 and kind1 >= 0 and kind1 <= 1 and kind2 >= 0 and kind2 <= 1",
		"not (kind0 == 0 or kind1 == 0 or kind2 == 0) or result == True",
		"not (kind0 == 0 or kind1 == 0 or kind2 == 0) or result == False",
	},
	{
		"sanitizer_call_always_clears",
		"def is_tainted(is_sanitizer: int, kind0: int, kind1: int):\n" +
			"    def leaf(k):\n        return k == 0\n" +
			"    arg_tainted = leaf(kind0) or leaf(kind1)\n" +
			"    if is_sanitizer == 1:\n        result = False\n" +
			"    else:\n        result = arg_tainted\n" +
			"    return result\n",
		"is_sanitizer >= 0 and is_sanitizer <= 1 and kind0 >= 0 and kind0 <= 1 and kind1 >= 0 and kind1 <= 1",
		"not (is_sanitizer == 1) or result == False",
		"not (is_sanitizer == 1) or result == True",
	},
	{
		"non_sanitizer_call_propagates",
		"def is_tainted(is_sanitizer: int, kind0: int, kind1: int):\n" +
			"    def leaf(k):\n        return k == 0\n" +
			"    arg_tainted = leaf(kind0) or leaf(kind1)\n" +
			"    if is_sanitizer == 1:\n        result = False\n" +
			"    else:\n        result = arg_tainted\n" +
			"    return result\n",
		"is_sanitizer >= 0 and is_sanitizer <= 1 and kind0 >= 0 and kind0 <= 1 and kind1 >= 0 and kind1 <= 1",
		"not (is_sanitizer == 0 and (kind0 == 0 or kind1 == 0)) or result == True",
		"not (is_sanitizer == 0 and (kind0 == 0 or kind1 == 0)) or result == False",
	},
	{
		"subscript_known_chain_tainted",
		"def is_tainted(chain_id: int):\n    return chain_id >= 0 and chain_id <= 4\n",
		"chain_id >= 0 and chain_id <= 9",
		"not (chain_id >= 0 and chain_id <= 4) or result == True",
		"not (chain_id >= 0 and chain_id <= 4) or result == False",
	},
	{
		"subscript_unknown_chain_clean",
		"def is_tainted(chain_id: int):\n    return chain_id >= 0 and chain_id <= 4\n",
		"chain_id >= 0 and chain_id <= 9",
		"not (chain_id >= 5) or result == False",
		"not (chain_id >= 5) or result == True",
	},
	{
		"augassign_clean_stays_clean",
		"def is_tainted(current_tainted: bool, value_tainted: bool):\n" +
			"    return current_tainted or value_tainted\n",
		"(current_tainted == True or current_tainted == False) and (value_tainted == True or value_tainted == False)",
		"not (current_tainted == False and value_tainted == False) or result == False",
		"not (current_tainted == False and value_tainted == False) or result == True",
	},
	{
		"augassign_either_tainted_propagates",
		"def is_tainted(current_tainted: bool, value_tainted: bool):\n" +
			"    return current_tainted or value_tainted\n",
		"(current_tainted == True or current_tainted == False) and (value_tainted == True or value_tainted == False)",
		"not (current_tainted == True or value_tainted == True) or result == True",
		"not (current_tainted == True) or result == False",
	},
	{
		"multihop_taint_survives_reassignment",
		"def is_tainted(source_tainted: bool):\n" +
			"    x = source_tainted\n    y = x\n    z = y or False\n    return z\n",
		"True",
		"not (source_tainted == True) or result == True",
		"not (source_tainted == True) or result == False",
	},
	{
		"multihop_clean_stays_clean",
		"def is_tainted(source_tainted: bool):\n" +
			"    x = source_tainted\n    y = x\n    z = y or False\n    return z\n",
		"True",
		"not (source_tainted == False) or result == False",
		"not (source_tainted == False) or result == True",
	},
	{
		"parameter_shadowing_blocks_outer_taint",
		"def is_tainted(outer_x_tainted: bool, is_shadowed_param: int):\n" +
			"    if is_shadowed_param == 1:\n        inner_x_tainted = False\n" +
			"    else:\n        inner_x_tainted = outer_x_tainted\n" +
			"    return inner_x_tainted\n",
		"is_shadowed_param >= 0 and is_shadowed_param <= 1",
		"not (is_shadowed_param == 1) or result == False",
		"not (is_shadowed_param == 1) or result == True",
	},
	{
		"non_shadowed_name_inherits_outer_taint",
		"def is_tainted(outer_x_tainted: bool, is_shadowed_param: int):\n" +
			"    if is_shadowed_param == 1:\n        inner_x_tainted = False\n" +
			"    else:\n        inner_x_tainted = outer_x_tainted\n" +
			"    return inner_x_tainted\n",
		"is_shadowed_param >= 0 and is_shadowed_param <= 1",
		"not (is_shadowed_param == 0 and outer_x_tainted == True) or result == True",
		"not (is_shadowed_param == 0 and outer_x_tainted == True) or result == False",
	},
	{
		"aliased_sanitizer_still_recognized_safe",
		"def is_tainted(resolves_to_sanitizer: int):\n" +
			"    if resolves_to_sanitizer == 1:\n        return False\n    return True\n",
		"resolves_to_sanitizer >= 0 and resolves_to_sanitizer <= 1",
		"not (resolves_to_sanitizer == 1) or result == False",
		"not (resolves_to_sanitizer == 1) or result == True",
	},
}

func TestOracle_TaintCheckerBoundedRules(t *testing.T) {
	python := testOraclePython(t)
	for _, c := range taintBoundedCases {
		t.Run(c.name+"/true_claim_proves", func(t *testing.T) {
			v, err := oracle.Prove(python, c.src, c.trueClaim, c.requires)
			if err != nil {
				t.Fatalf("Prove: %v", err)
			}
			if v.Status != "PROVED" {
				t.Fatalf("got status %q, want PROVED: %+v", v.Status, v)
			}
		})
		t.Run(c.name+"/false_claim_refutes", func(t *testing.T) {
			v, err := oracle.Prove(python, c.src, c.falseClaim, c.requires)
			if err != nil {
				t.Fatalf("Prove: %v", err)
			}
			if v.Status != "REFUTED" {
				t.Fatalf("got status %q, want REFUTED (bounded model may be rubber-stamping): %+v", v.Status, v)
			}
			if v.Counterexample == "" {
				t.Error("REFUTED verdict missing a counterexample")
			}
		})
	}
}
