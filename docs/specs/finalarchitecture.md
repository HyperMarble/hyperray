# Hyperray v0.10 Final Architecture

Status: **FROZEN — 2026-08-27**

This is Hyperray v0.10. It replaces the rejected broader draft. Its scope is only
to determine whether one fixed coding task or PR is actually correct.

## 1. Result Hyperray must establish

For one exact task/PR, Hyperray must prove:

1. The exact reference solution implements the required logic for every case
   inside the task's complete bounded scope.
2. No incorrect solution behavior can pass the task's tests. This eliminates
   false positives.
3. Correct solution behavior is not rejected by the tests. This eliminates
   false negatives and unfair hidden requirements.
4. The exact reference solution passes the verifier over the complete bounded
   scope.

Hyperray verifies the task before coding agents are evaluated. Agent success,
failure, sampling, or mutation score is not the proof.

## 2. Fixed inputs

One run freezes:

- the instruction or PR/issue description;
- the exact base repository commit;
- `spec.md`;
- the reference solution/diff;
- public and hidden verifier tests;
- the test command and authoritative pass signal;
- the language toolchain and dependencies; and
- the execution environment.

The same architecture applies to benchmark tasks and normal PRs. A PR uses its
issue/PR description as the instruction, its parent commit as the base, its diff
as the reference solution, and its test changes as the verifier.

## 3. spec.md

`spec.md` is Hyperray's machine-readable input for the task's bounded behavior. It
is parsed by Hyperray; it is not a PRD or a file the coding agent must follow.

The task author writes it using the spec skill after reading the instruction,
base code, issue/PR, reference diff, and relevant environment behavior. The
instruction gives the contract -- what behavior is owed; the reference gives
the mechanism -- the exact conditions, value lists, and outcomes. A row exists
because the contract owes it, and its `Evidence` cell anchors into either
artifact: a bare span cites the instruction, `reference:<span>` cites the
reference solution. Behavior the reference implements that the contract never
owes is an implementation choice and is not graded. The frozen statement of
this is `evidence-rule.md` in this directory. Tests do not decide the
requirements. They are compared against the requirements after the semantic
rows are fixed.

`spec.md` declares:

- bounded parameters and their exact domains;
- constraints excluding impossible cases;
- the required observable behavior for every full N-way parameter
  combination; and
- relevant returns, exceptions, values, data types, shapes, labels, state
  changes, calls, and side effects.

Hyperray's strict parser expands compact rows into the complete full N-way set.
`spec-lint` proves the tables are complete, disjoint, and use only declared
values.

## 4. Finite proof model

Let:

```text
D = the complete finite set of task inputs and states
O = the complete finite set of observable outcomes
R(x, o) = outcome o is correct for case x according to compiled spec.md
C(x, o) = the reference solution can produce outcome o for case x
T(F) = the frozen verifier passes implementation behavior F
F : D → O = one complete possible implementation behavior
```

The formal layer covers every element of `D` and every relevant outcome in
`O`. It may enumerate them or represent them exactly with SAT/SMT/BDD. It may
not sample them.

Because accepted Hyperray tasks have a closed finite model, these checks are
decidable. General undecidability for unrestricted programs is outside the
accepted task format.

### 4.1 A bound must be proved sufficient, not asserted

A declared bound being finite does not make it correct. A domain can be finite
and still exclude a reachable case, and the resulting `VERIFIED` then covers a
smaller problem than the real one — the same defect Hyperray exists to remove,
relocated one level up.

Bounded model checkers do not trust a caller-supplied bound either. They
assert it: an unwinding assertion at each loop fails when the bound was too
small, and a computed completeness threshold is the `k` beyond which no new
counterexample can exist. Hyperray requires the equivalent for every declared
domain — each category reachable, and no reachable behavior outside the
declared categories. An unproved bound yields `PROOF BLOCKED`.

### 4.2 Depth-dependent bounds

Some tasks bound a loop by input size rather than by a constant. Measured on
the accepted set, every loop is either a `for` over a finite structure or a
worklist that drains; none is syntactically unbounded. Two of seven Shipd
tasks are stack-safety tasks whose requirement *is* depth, and whose tests
exercise depth 2500.

Full unrolling is not the answer there. k-induction discharges such a loop
without it: a base case over `k` unwindings, a forward condition that the
loops were fully unrolled within `k`, and an inductive step showing that
holding at `k` implies holding at `k+1`. The bound then stops appearing in
the proof. ESBMC implements this directly.

The residual cost is the loop invariant. It may be proposed by the task
author or a model and must be discharged by the backend; a proposed invariant
the backend rejects is not evidence.

## 5. Mandatory proofs

### 5.1 Reference solution correctness

Hyperray proves:

```text
For every x in D:
    the reference solution's outcome satisfies R
```

Counterexample query:

```text
EXISTS x, o: C(x, o) AND NOT R(x, o)
```

Required result: `UNSAT`.

### 5.2 False-positive freedom

A false positive is an incorrect candidate solution that passes the tests.

Hyperray proves:

```text
For every complete implementation behavior F:
    T(F) implies that every F(x) satisfies R
```

Counterexample query:

```text
EXISTS F: T(F) AND EXISTS x: NOT R(x, F(x))
```

Required result: `UNSAT`.

A satisfying behavior is the exact proof of a test gap: it passes the verifier
while failing at least one required case.

### 5.3 False-negative freedom and test fairness

A false negative is a correct solution behavior rejected by the tests.

Hyperray proves:

```text
For every complete implementation behavior F:
    if every F(x) satisfies R, then T(F)
```

Counterexample query:

```text
EXISTS F: every F(x) satisfies R AND NOT T(F)
```

Required result: `UNSAT`.

This catches hidden tests that enforce behavior absent from the task's stated
requirements, overly rigid type or implementation checks, and other unfair
restrictions.

### 5.4 Exact reference acceptance

Section 5.1 proves that the exact reference produces a permitted result for
every case in `D`. Separately, Hyperray proves that the complete frozen verifier
accepts that exact reference:

```text
T(C) = true
```

Both checks are mandatory. A passing verifier run cannot replace the proof over
every case in `D`, and the proof over `D` cannot replace running the complete
verifier.

## 6. Prompt, spec, and hidden-test alignment

Every semantic row in `spec.md` must have provenance in a frozen artifact:
the instruction for the contract that owes the row, or the reference for the
mechanism that shapes it, per `evidence-rule.md`. Tests never supply
provenance, so hidden tests cannot silently add unstated requirements, and a
reference-anchored row remains subject to the contract test: mechanism the
contract does not owe stays ungraded.

The two set-inclusion proofs expose both disagreement directions:

```text
prompt/spec requirement not enforced by tests → false-positive witness
test restriction not permitted by prompt/spec → false-negative witness
```

If hidden tests enforce something the prompt never states, Hyperray reports the
second direction. The author must either remove/fix that hidden test or state
the intended requirement and regenerate the frozen spec.

## 7. Exact semantic inputs to the proof

Hyperray independently builds:

1. Spec semantics from parsed `spec.md` tables.
2. Reference-solution semantics from the real solution and language verifier.
3. Test semantics from the real test/verifier code and pass signal.
4. Environment semantics from the frozen runtime, dependencies, and bounds.

They are normalized into one Semantic IR only after independent extraction.
Expected results from the spec cannot be copied into the solution or test
translation to manufacture agreement.

Python, Rust, and C++ use language-specific compiler/model-checker frontends.
The task format requires finite bounds and any annotations or harness needed by
those tools. Unsupported or incomplete translation cannot return `VERIFIED`.

### 7.1 Frontends are translators, not provers

A verifier is not built per language. An intermediate verification language
decouples the frontend, which encodes one language's semantics, from the
backend, which automates proof search. Backends are reused across languages,
so adding a language means writing a translator, not a solver.

Two backends already cover Hyperray's targets:

| language | frontend | backend |
|---|---|---|
| Python | Nagini | Viper |
| Go | Gobra | Viper |
| Rust | Prusti | Viper |
| Rust | Kani | CBMC |
| C, C++ | ESBMC, CBMC | own SMT |
| Java | JBMC | CBMC |

Viper is therefore the existing realization of a shared semantic layer for
Python, Go, and Rust; Hyperray does not need to invent one for those languages. A
language that lowers to LLVM IR (Zig, and C/C++/Rust already) is reachable
through the LLVM-based backends without a bespoke frontend.

These tools verify *annotated* programs: the caller supplies preconditions,
postconditions, and loop invariants, and the tool proves them. That is
compatible with Hyperray by construction, because compiled `spec.md` is precisely
where those annotations come from. It also fixes the boundary: Hyperray supplies
the specification, the frontend supplies the program semantics, and the
backend supplies the proof. Hyperray does not re-derive any of the three.

Selecting a frontend does not weaken section 4. A frontend that cannot
translate reachable task behavior yields `PROOF BLOCKED`, never `VERIFIED`.

## 8. Existing Hyperray layers

The existing layers remain and feed the final proof:

1. `spec-lint` checks machine-readable table completeness and disjointness.
2. `coverage`/PICT generates t-way scenarios and reports likely missing tests.
3. `oracle` formally proves the simplified mathematical reference model
   against the compiled spec behavior.
4. `diff-test` compares the real reference solution with the proven oracle.
5. `dep-harvest` supplies relevant dependency edge values before domains are
   frozen.
6. Semantic IR plus SAT/SMT performs the complete reference, false-positive,
   and false-negative proofs and is the only mathematical all-clear.

PICT, mutation testing, property-based testing, fuzzing, coverage, and
differential samples may find counterexamples. A clean result from any of them
does not prove that no counterexample exists.

## 9. Verifier protection

The verifier, hidden tests, and pass signal are frozen and outside candidate
write access. Candidate code cannot modify the grader or its result. The exact
environment and tool versions used to derive the proof are the ones used to
confirm it.

These protections ensure `T(F)` represents the real verifier rather than a
candidate-forged result. They do not replace the three mathematical proofs.

## 10. Counterexamples and repair

Hyperray returns exact witnesses:

- reference bug — a bounded case where the reference result is wrong;
- false positive — an incorrect complete behavior that still passes tests;
- false negative — correct behavior rejected by tests; or
- prompt/test mismatch — a hidden restriction not stated in the task.

Each witness is executed in the frozen real environment. The task author fixes
the solution, prompt/spec, or tests and reruns the complete pipeline.

## 11. Verdicts

Hyperray returns:

- `VERIFIED` — all mandatory proofs are complete and have no counterexample;
- `NOT VERIFIED` — at least one proof has a confirmed counterexample; or
- `PROOF BLOCKED` — the bounded model, translation, environment, or solver
  evidence is incomplete.

Skipped analysis, `UNKNOWN`, sampling success, killed mutants, pairwise
coverage, or a passing reference test run can never produce `VERIFIED`.

## 12. v0.10 completion

Hyperray v0.10 is complete when `hyperray start` and `hyperray check` run this same pipeline
end to end for real Python, Rust, and C++ tasks/PRs and demonstrate:

1. a correct reference and correct tests are `VERIFIED`;
2. an incorrect reference produces a reference counterexample;
3. a missing requirement check produces a false-positive counterexample;
4. an unstated hidden-test requirement produces a false-negative
   counterexample;
5. the exact reference passes every case in the complete bounded scope; and
6. stale, incomplete, unsupported, or tampered evidence cannot receive
   `VERIFIED`.

That is the entire v0.10 product. Additional features belong to later versions.
