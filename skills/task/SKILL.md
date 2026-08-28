---
name: task
description: Author or audit one fixed bounded Python, Rust, or C++ task/PR for Ray's exact reference, false-positive, false-negative, and verifier-acceptance checks.
---

# Build a formally verifiable task

Ray evaluates one frozen coding task or PR before coding agents are scored. A
task is ready only when the same fail-closed pipeline used by `ray start` and
`ray check` returns `VERIFIED` with complete frozen evidence.

Use the [spec skill](../spec/SKILL.md) to author `spec.md`. It is Ray's strict
machine-readable source of complete finite behavior and compiles into Spec
Semantic IR. The candidate does not read it.

## 0. The problem statement is written by a human voice

Reviewers reject the telegraphic spec-dump style on sight -- every noun
backticked, staccato contract recitation, prose that reads machine-written.
Write the statement as a maintainer's engineering ticket: a motivation
sentence, flowing paragraphs, one requirement per sentence, with every exact
error fragment and API name still embedded verbatim. Precision and voice are
not in tension: the extractor needs the exact fragments; the human reviewer
needs the prose around them to sound like a person. Verify parity after any
restyle: every quoted fragment and backticked term of the old text must
appear in the new, and spec Evidence anchors must be remapped to the new
line numbers. The statement is plain ASCII only: no em dashes, no curly
quotes -- the platform rejects the first non-ASCII byte it finds, and prose
style is the usual way one sneaks in. A restyle is a contract edit: any
phrasing that strengthens a claim ("tuple input works" becoming "any
sequence type should work") creates a requirement the extractor will grade,
so after restyling, re-extract and diff the requirement inventory. A new
or strengthened line is a SCOPE CHANGE, and scope belongs to the task
author alone: never widen, narrow, or add a claim without the author's
explicit approval -- surface the diff and ask. Only author-approved
additions then get tests before the wording ships.

## 1. Fix the task before tests

Freeze and inspect, in this order:

1. Complete instruction, issue, or PR description with stable lines.
2. Exact base repository commit and relevant unchanged code/callers.
3. Complete reference solution or PR diff and the applied reference tree.
4. Relevant pinned environment, dependencies, flags, toolchain, and bounds.

Do not open public or hidden tests, test patches, test names, grader scripts, or
test-derived logs during this review.

The instruction gives the contract; the reference diff gives the mechanism. A
row exists because the contract owes that behaviour, and the reference decides
its exact conditions, value lists, and outcomes. Behaviour the reference
implements that the contract never asks for is an implementation choice and is
not graded. See [the evidence rule](../../docs/specs/evidence-rule.md), which
is frozen.

For a benchmark task, reconstruct base, base-plus-tests, and
solution-plus-tests from the declared commit and patches. During the pre-test
semantic review, apply only the reference patch in a separate workspace. For a
normal PR, use its parent as base, the PR diff as reference, and its public and
hidden test changes as the verifier only after semantics freeze.

Record exact artifact paths, commits, and digests. A workspace that cannot be
reproduced from those inputs blocks the task.

## 2. Author complete bounded behavior

Follow the spec skill to define:

- operation-local bounded parameters and exact finite domains;
- concrete impossible-case constraints;
- every complete full-N-way input/state combination;
- exact required returns, exceptions, values, types, shapes, labels, effects,
  calls, outputs, mutations, and state transitions;
- simultaneous-clause precedence, ordering, shared state, repeated calls, and
  other relevant interactions; and
- exact instruction/issue provenance for every semantic row.

Every phase-1 `Enforced by` cell is `none`. Structural `spec-lint` must pass,
then record and preserve the reviewed spec bytes and digest before opening
tests. If required behavior is unbounded or cannot be represented exactly by
the strict language and supported frontend, return `PROOF BLOCKED`; do not
sample a smaller task without changing the instruction.

Audit pinned dependencies before the final domain freeze. `dep-harvest` must
provide relevant dependency edge values for author review when such a
dependency exists. Record it as `N/A` only with evidence that the operation has
no relevant dependency. Add accepted edge values to `spec.md`, rerun full-N-way
expansion, and freeze the revised domains again.

## 3. Compare public and hidden tests separately

After the semantic freeze, inspect the actual public and hidden tests, runner,
setup/teardown, shared state, ordering, test command, and authoritative pass
signal. Update only `Enforced by` with stable Test IR IDs. Keep `none` for an
unobserved or uncertain row.

The tests form one exact global predicate `T(F)` over the full implementation
behavior, not independent per-row checks. Preserve cross-case comparisons,
chained calls, fixtures, shared state, ordering, and the final pass/fail
calculation.

Complete the mapping-review template and check both directions:

- False positive: an incorrect complete behavior passes the verifier.
- False negative: a behavior permitted by the frozen requirements is rejected
  by public or hidden tests.

An unstated hidden requirement is a false-negative/fairness defect. Fix the
test or restart semantic authoring and explicitly add the intended requirement;
never smuggle it into the existing freeze.

## 4. Supply the simplified oracle and comparison inputs

Provide or derive a simplified mathematical oracle for the required logic. Its
source, assumptions, finite inputs, tool identity, and digest are mandatory
frozen inputs. The language formal verifier must establish that the oracle
satisfies every bounded Spec IR case; failure, partial translation, or solver
`UNKNOWN` blocks the task.

Record the real reference entry points and the exact oracle/reference inputs
used by `diff-test`. Differential agreement on generated edge cases is useful
supporting evidence; disagreement is a reference bug. Agreement never replaces
the complete formal reference check.

PICT scenarios must be derived from the frozen Spec parameter domains. Do not
author a separate PICT domain model or use t-way scenarios as the complete
behavior set.

## 5. Freeze exact production inputs

The production run freezes:

- instruction or issue/PR description;
- exact base commit;
- final `spec.md` and its pre-test semantic digest;
- reference solution/diff;
- public and hidden verifier tests;
- verifier command and authoritative pass signal;
- language tools and dependencies; and
- execution environment.

The freeze also binds the simplified oracle, its verifier inputs, reviewed
dependency-harvest output or justified `N/A`, PICT configuration derived from
Spec domains, and exact diff-test inputs.

Configuration identifies artifacts, tools, workspaces, commands, and bounds.
It cannot define a missing requirement, outcome, effect, constraint, or test
predicate. Hash or tool drift invalidates earlier evidence.

## 6. Run the exact pipeline

The retained Ray layers have distinct roles:

```text
spec-lint -> coverage/PICT -> oracle -> diff-test -> dep-harvest
          -> independent reference and real-test translation
          -> exact Semantic-IR checks -> real witness confirmation
```

- `spec-lint` checks strict full-N-way table structure.
- `dep-harvest` supplies reviewed dependency edge values before final domain
  freeze, or carries the evidenced no-relevant-dependency `N/A` finding.
- PICT consumes the frozen Spec domains; it does not define independent bounds.
- `coverage`/PICT, mutation, fuzzing, property-based testing, and differential
  samples can find likely gaps or witnesses.
- The mandatory simplified oracle is checked against every compiled Spec IR
  case.
- `diff-test` compares the real reference with the proven oracle on its exact
  frozen comparison inputs.
- Language frontends translate the real reference and the real public/hidden
  verifier independently.
- Test IR preserves one global verifier predicate.
- Exact proof checks cover the complete bounded behavior space.
- Every counterexample is confirmed in a fresh frozen environment.

Diagnostic success cannot produce `VERIFIED` or compensate for a missing
translation. Do not replace the actual verifier with test logic synthesized
from the requirements; doing so erases the questions Ray is meant to answer.

## Mandatory results

For finite inputs/states `D`, outcomes `O`, required relation `R`, exact
reference relation `C`, implementation behavior `F : D -> O`, and real verifier
predicate `T(F)`, require:

```text
Reference correctness:
  EXISTS x,o: C(x,o) AND NOT R(x,o)                         = UNSAT

False-positive freedom:
  EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))                = UNSAT

False-negative freedom:
  EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)              = UNSAT

Exact reference acceptance:
  T(C)                                                       = true
```

The first and fourth results are separate: a correct bounded reference can
still fail an unfair verifier, and a passing reference run cannot establish
correctness for every case.

## Verdicts

- `VERIFIED` — all mandatory exact results are complete, every translation and
  artifact is fresh, and no counterexample exists.
- `NOT VERIFIED` — an exact reference, false-positive, or false-negative
  witness is confirmed.
- `PROOF BLOCKED` — bounds, translation, environment, artifact, solver, or
  execution evidence is incomplete or unknown.

Both commands use the same production path and exit nonzero unless verified:

```sh
ray check <task-folder>
ray start <task-folder>
```

## Final task review

1. The fixed instruction/issue, base, reference diff, relevant environment,
   and patch-shaped workspace provenance are complete.
2. Semantic rows were authored and digested before public or hidden tests were
   accessed.
3. Domains are exact and finite; every local full-N-way assignment is reachable
   or has a justified impossible-case constraint.
4. Outcomes, effects, shapes, ordering, and state are complete.
5. Public and hidden tests are translated independently into one global
   predicate, with mappings recorded but not treated as proof truth.
6. Reference correctness, false-positive freedom, false-negative freedom, and
   exact reference acceptance all complete.
7. No diagnostic, sample, score, skipped stage, or solver `UNKNOWN` is promoted
   to `VERIFIED`.
