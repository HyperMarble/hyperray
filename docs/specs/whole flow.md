# Ray v0.10 Whole Flow

Status: **FROZEN — 2026-08-27**

This is the complete operational flow for `finalarchitecture.md`.

## 1. Author the task or PR specification

Input:

```text
instruction or issue/PR description
exact base commit
reference solution/diff
relevant code paths and dependencies
frozen environment
```

The human or AI author uses the spec skill to write `spec.md`:

```text
bounded parameters
exact domains
impossible-case constraints
required behavior for every full N-way combination
instruction/PR provenance for every requirement
```

Strict spec lint expands the tables and rejects gaps, overlaps, undeclared
values, ambiguous behavior, or unsupported unbounded domains.

The semantic rows are frozen before tests are used as enforcement evidence.

## 2. Attach and inspect the tests

Ray freezes public and hidden tests, their runner, and the authoritative pass
signal.

The author records which tests are intended to enforce each already-frozen
requirement. Missing mappings stay missing. Tests cannot silently create a new
requirement.

If reviewing tests reveals that the intended requirement itself was written
incorrectly, the author restarts spec authoring and creates a new semantic
freeze.

## 3. Freeze the exact task

Ray freezes and hashes:

```text
instruction / PR description
spec.md
base repository and commit
solution diff
test diff
test command and pass signal
language tools and dependencies
execution environment
```

For a benchmark task, Ray reconstructs the base, base-plus-tests, and
solution-plus-tests states from the exact commit and patches.

For a normal PR, Ray uses the PR parent as the base, the PR diff as the
reference solution, and the PR tests as the verifier change.

An artifact or workspace that cannot be reproduced from the frozen inputs
blocks the run.

## 4. Compile the machine-readable spec

```text
spec.md
   ↓ parse
strict behavior tables
   ↓ expand
complete full N-way case set
   ↓ compile
Spec Semantic IR
```

The compiler emits:

- exact finite domains;
- every bounded input/state case;
- constraints;
- required returns, exceptions, values, shapes, labels, effects, and state;
- instruction/PR provenance; and
- a canonical digest.

No proof stage reads Markdown directly.

## 5. Enrich bounded domains before final proof

`dep-harvest` may obtain relevant edge values from the exact pinned dependency
version. The author reviews and adds accepted values to `spec.md`, after which
the complete domains and Spec IR are frozen again.

PICT consumes the frozen parameter domains and generates configured t-way test
scenarios. It reports likely coverage gaps but does not provide the formal
all-clear.

## 6. Prove the simplified oracle

The task supplies or derives a simplified mathematical oracle for the required
logic.

```text
Spec Semantic IR + simplified oracle
                  ↓
        language formal verifier
                  ↓
oracle satisfies every bounded spec case
```

Python, Rust, and C++ use their supported formal-verification frontends and
SAT/SMT backends. A failed, partial, or unknown oracle proof blocks the run.

## 7. Translate the exact reference solution

The language frontend translates the real reference solution under the frozen
toolchain and environment.

It must represent every task-relevant input, branch, loop bound, return,
exception, effect, and state transition. Unsupported reachable behavior blocks
the run rather than being omitted.

`diff-test` compares the real reference solution against the proven oracle on
generated edge cases. Any disagreement is a concrete reference-solution bug.
Agreement is supporting evidence; the complete formal reference proof remains
required.

## 8. Translate the exact verifier

Ray translates the public and hidden tests plus their pass-signal calculation
into one global Test Semantic IR predicate.

The predicate preserves:

- every test that affects grading;
- all assertions and exact expected observations;
- setup, teardown, ordering, and shared state;
- chained operations and feature interactions;
- cross-case comparisons; and
- the final pass/fail calculation.

Ray does not infer enforcement from test names, shared words, coverage
percentage, or mutation score.

## 9. Build the complete bounded behavior space

Let:

```text
D = every bounded input/state case from Spec IR
O = every relevant observable outcome
F : D → O = one complete possible implementation behavior
R(x, o) = expected behavior from Spec IR
C(x, o) = exact reference behavior
T(F) = exact verifier result from Test IR
```

Ray represents all possible `F` exactly, using enumeration or a symbolic
SAT/SMT/BDD formula. It never substitutes pairwise combinations, sampled
inputs, mutants, or randomized cases for this complete behavior space.

## 10. Prove the exact reference solution

Ray asks whether the reference can produce any incorrect bounded result:

```text
EXISTS x, o: C(x, o) AND NOT R(x, o)
```

Expected result:

```text
UNSAT → reference logic is correct for every bounded case
SAT   → exact reference counterexample
```

The query above covers every case in `D`. Separately, Ray runs the complete
frozen verifier against the exact reference solution and proves:

```text
T(C) = true
```

Neither result substitutes for the other.

## 11. Prove there are no false positives

A false positive is an incorrect solution behavior that passes the tests.

Ray asks:

```text
EXISTS complete behavior F:
    T(F)
    AND
    EXISTS x in D where NOT R(x, F(x))
```

Expected result:

```text
UNSAT → no incorrect bounded behavior can pass
SAT   → exact silently-passing incorrect behavior
```

The satisfying assignment identifies the missing requirement check and the
specific wrong result that the existing tests accept.

## 12. Prove there are no false negatives

A false negative is correct behavior rejected unfairly by the tests.

Ray asks:

```text
EXISTS complete behavior F:
    every F(x) satisfies R
    AND
    NOT T(F)
```

Expected result:

```text
UNSAT → tests accept every permitted bounded behavior
SAT   → exact unfairly-rejected correct behavior
```

This proof catches:

- hidden tests enforcing requirements absent from the prompt/spec;
- exact-type or internal-implementation checks not required by the task;
- incorrect floating-point strictness;
- unstated ordering, shape, label, or exception requirements; and
- other over-restrictive assertions.

## 13. Confirm every finding

For each SAT witness, Ray runs the exact counterexample in the frozen real
environment.

```text
reference witness       → run the real reference
false-positive witness  → run the real verifier against the wrong behavior
false-negative witness  → run the real verifier against the permitted behavior
```

If execution confirms the model, the finding is proven. If execution disagrees
with the model, Ray reports a translation/model defect and blocks verification.

## 14. Repair and rerun

| Finding | Repair |
|---|---|
| reference counterexample | fix the reference solution or correct and refreeze the intended requirement |
| false-positive counterexample | strengthen the tests so the wrong behavior fails |
| false-negative counterexample | remove the unstated or overly rigid test restriction, or explicitly state and refreeze the intended requirement |
| hidden-test prompt gap | add the intended requirement to the prompt/spec or remove the hidden test requirement |
| incomplete translation | bring the task into Ray's fixed bounded profile or extend the language frontend generically |

Every repair reruns the entire pipeline. Proofs from changed hashes are stale.

## 15. Supporting diagnostics

Ray may also run:

```text
PICT coverage
mutation testing
property-based testing
fuzzing
dependency edge harvesting
differential samples
```

These layers can find useful counterexamples. None can prove that no false
positive remains. Only the complete formal proofs decide `VERIFIED`.

## 16. Final verdict

```text
VERIFIED
```

requires all of:

1. the complete bounded spec compiles;
2. the simplified oracle is formally proven;
3. the exact reference is correct for every bounded case;
4. the exact reference passes the complete verifier;
5. the false-positive query is `UNSAT`;
6. the false-negative query is `UNSAT`;
7. all translations are complete;
8. every counterexample has been resolved by a fresh full run; and
9. frozen artifacts and evidence still match.

Otherwise Ray returns:

```text
NOT VERIFIED  → confirmed reference, false-positive, or false-negative witness
PROOF BLOCKED → incomplete bounded model, translation, environment, or solver
```

## 17. Complete pipeline

```text
fixed task or PR
      ↓
author machine-readable spec.md from instruction/base/reference/environment
      ↓
freeze semantics, then compare public and hidden tests
      ↓
spec-lint full N-way completeness/disjointness
      ↓
dep-harvest reviewed edge values, then freeze final domains
      ↓
PICT coverage diagnostics
      ↓
compile Spec Semantic IR
      ↓
prove simplified oracle
      ↓
compile exact reference semantics
      ↓
diff-test supporting comparisons
      ↓
compile exact global Test Semantic IR
      ↓
build complete bounded implementation-behavior space
      ↓
prove exact reference correctness
      ↓
prove no incorrect behavior passes
      ↓
prove no correct behavior fails unfairly
      ↓
execute every counterexample in the frozen environment
      ↓
repair and rerun until every mandatory query is UNSAT
      ↓
VERIFIED task/PR
```

## 18. v0.10 end-to-end acceptance

Production `ray start` and `ray check` must execute this same flow on real
Python, Rust, and C++ tasks/PRs.

Required demonstrations:

1. correct solution plus correct verifier → `VERIFIED`;
2. incorrect reference logic → exact reference witness;
3. prompt requirement omitted by tests → exact false-positive witness;
4. hidden test requirement omitted by prompt → exact false-negative witness;
5. overly rigid test assertion → exact false-negative witness;
6. exact reference passes every complete bounded case;
7. stale, unsupported, partial, or tampered evidence never returns
   `VERIFIED`.

That completes Ray v0.10. Later versions may add capabilities without changing
this proof goal.
