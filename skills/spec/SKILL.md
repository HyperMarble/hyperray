---
name: spec
description: Author or revise Hyperray's strict machine-readable spec.md for one fixed bounded coding task or PR, deriving complete finite behavior before tests and comparing public and hidden tests only after the semantic rows are frozen.
---

# Author a complete bounded spec

`spec.md` is Hyperray's strict machine-readable source for a task's complete finite
behavior. Hyperray compiles it into Spec Semantic IR. Coding agents do not read it,
and proof stages do not infer requirements from tests.

[references/schema.md](references/schema.md) is the exact input contract the
compiler accepts — directives, columns, witness form and order, the closed
outcome and effect vocabularies, and every diagnostic with its cause. Read
it before writing a row; anything outside it is rejected at ingestion rather
than repaired.

[references/examples.md](references/examples.md) carries the two patterns
that every real task needs and that no column name suggests: a **relational**
requirement ("remains intact", "applying it again changes nothing") written
as an operation returning a bool, and a **non-scalar observable** (where an
instruction was placed, whether a connection was replaced) written as an
operation returning a label.

Start from [templates/spec.md](templates/spec.md). Use
[templates/authoring-record.md](templates/authoring-record.md) during the
test-blind review and [templates/mapping-review.md](templates/mapping-review.md)
after the semantic rows freeze. Read [references/examples.md](references/examples.md)
when you need current strict syntax examples.

## Evidence boundary

Use two distinct phases:

1. Derive and freeze task semantics without opening tests.
2. Inspect the frozen public and hidden tests only to map enforcement and
   compare the verifier with the already-frozen behavior.

During phase 1, do not open, search, run, summarize, or ask another agent to
inspect test files, test patches, test names, grader scripts, pass logs, or any
test-derived artifact. Tests never supply a parameter, domain value,
constraint, required outcome, effect, state transition, or provenance anchor.

If phase 2 reveals that the intended requirement was authored incorrectly,
discard the semantic freeze and restart phase 1. Do not edit a requirement to
make an existing test appear correct.

## Phase 1: derive semantics before tests

### Read the whole fixed task

Read and record exact artifact identities for:

1. The complete instruction, issue, or PR description with stable line
   numbers.
2. The declared base commit and relevant unchanged code, callers, and state.
3. The complete reference solution or PR diff and enough applied source to
   follow every changed branch, return, exception, call, effect, and preserved
   path.
4. Relevant environment facts: pinned dependencies, language/tool versions,
   feature flags, platform behavior, build settings, and external bounds that
   affect reachability or observations.

The instruction gives the contract; the reference gives the mechanism. Derive
a row because the contract owes that behaviour, then take its exact conditions,
value lists, and outcomes from the reference. Behaviour the reference
implements that the contract never asks for is an implementation choice: leave
it out, because grading it rejects a correct solution written differently.
Behaviour the contract owes that the reference omits is a reference bug, not a
row to soften. This is [the evidence rule](../../docs/specs/evidence-rule.md)
and it is frozen; read it before writing a row.

Record disagreements between instruction, base, diff, and environment. Resolve
them with the task owner or leave the task blocked.

For a patch-shaped benchmark, reconstruct the base from the declared commit
and apply only the reference patch in a separate workspace. Do not apply or
inspect the test patch. For a normal PR, use the parent commit as base and the
PR diff as the reference. Inspect changed functions, relevant callers,
unchanged neighboring branches, and dependency behavior; a hunk alone is not
enough to define scope.

### Fix operation scope and finite domains

Define each independently observable operation. For every operation, list all
task-relevant inputs, pre-state, bounded history, environment modes, and
interaction flags. Use operation-local domains; do not form a Cartesian
product between unrelated operations.

**The inventory comes first — never write domains from prose directly.**
Before any domain, itemize the contract into numbered testable assertions,
one distinction per line: every error message, every return shape, every
ordering rule, every conjunct of every "and" as its own line. Then build
domains from the inventory, one value per line, and record which lines each
row covers. A line no row covers is a named gap, not a silent one. Writing
domains straight from prose is where distinctions get compressed away; the
inventory makes every distinction visible before compression can happen.

**The counting pass — mandatory, never rushed.** After writing every
domain, reread the contract sentence its rows anchor to. Count the
distinctions the sentence makes; count the values the domain declares. The
numbers match, or the difference is justified in writing beside the domain.
The split criterion is decidable, not felt: split two phrases into two
lines **when testing them needs independent inputs or different trigger
conditions**; keep them one line when a single assertion checks both.
"Index and columns" splits (different inputs); "retains the object,
including its name" stays one (one assertion sees both). Quantifiers
("every horizon over one", "regardless of order") are one rule whose domain
carries one value per case -- never one collapsed row, never N separate
rules. A conjunction is a count: "values and order" is two distinctions;
"index and columns" doubles them; "rows and columns", "overall and
in-sample" likewise.
One author lost three requirements in one task by translating the word
instead of counting the distinctions -- each found later by an outside
reader, each invisible to every downstream check, because a distinction the
spec never names is a question no machine can ask.

Every quantifier in the contract is a domain, never a label. When the
contract says "every horizon over one", "any order", "all components", the
quantified variable becomes a parameter with a `Universe:` of representative
values and one row per value -- not a single true/false aspect row. A lone
boolean hides the dimension, so no later stage can ask whether each value is
enforced; a span tested only at 3 stays invisible exactly this way. The
compiler warns (`quantifier-as-label`) when a bool row anchors into
quantifying contract text.

**The divergent witness — every agreement promise needs one.** A sentence
promising two outputs agree ("equals", "agrees", "matches", "same as") is
only proven by a scenario where the two sides COULD disagree. If both sides
derive from one source, agreement is automatic and the test proves nothing:
a wrapper's `predict_var` "agreed" with its composed variance for weeks
because every test forecaster derived its own variance from its own
distribution — one divergent forecaster (distribution says 1, direct method
says 4) exposed the promise as unimplemented. For each agreement row, add a
domain value whose witness makes the sources diverge, and require the
promised side to win.

Declare each domain with a finite, disjoint value list:

```markdown
Parameters: `target_state` ("writable" / "read-only"), `payload_kind` ("valid" / "invalid").
```

JSON strings are the unambiguous lexical encoding for semantic values. The
exact token ` / ` separates values only outside quotes. Paths, URLs, dates,
Unicode, embedded slashes, quotes, and backslashes therefore remain one value.
The unquoted token `any` is a row wildcard; the quoted string `"any"` is an
ordinary domain value.

Use current strict typed bindings when the operation consumes concrete values:

```text
Inputs: persist(target: string, payload: string).
Grounding: persist.target_state."writable" = when target == "writable"; witness {"payload":"valid","target":"writable"}.
Grounding: persist.target_state."read-only" = when target == "read-only"; witness {"payload":"valid","target":"read-only"}.
```

Groundings are authored from phase-1 evidence, never from test literals. Every
operation/domain/value triple has one closed membership expression and a
canonical satisfying witness. If the strict language or a language frontend
cannot represent a necessary finite input, state, shape, or history, the task
is `PROOF BLOCKED`; do not replace the missing behavior with samples.

### Expand the complete full-N-way behavior

Take the full Cartesian product of every domain local to an operation. For
every expanded assignment, write exactly one row:

- `reachable` when it is a real bounded case;
- `excluded` only when phase-1 evidence proves it impossible.

Every excluded row needs a precise `Constraint reason` and source evidence.
Constraints remove impossible cases; they cannot hide a difficult, untested,
or unsupported case.

Full-N-way means simultaneous clauses remain simultaneous. Review validation
precedence, multiple faults, shared state, ordering, repeated calls, loops,
setup/teardown state, and one clause preserving behavior while another changes
it. Pairwise or t-way coverage is not a substitute for these rows.

### Describe exact observable behavior

Each reachable row declares the complete permitted behavior for that full
assignment:

- return or success result, exact value, type, label, and shape;
- raised exception type and relevant message;
- ordered or unordered effects as required by the operation semantics;
- reads, writes, calls, outputs, mutations, and state transitions;
- permitted alternatives, including nondeterministic alternatives explicitly
  allowed by the instruction; and
- forbidden outcomes needed to close the operation's observable alphabet.

Use strict outcome forms supported by the compiler:

```text
success
timeout
return unit
return null
return true
return false
return 42
return "stored"
raise ValidationError containing "invalid payload"
other outcome
return unit with call:engine.find, write:phase="Sending"
```

`other outcome` is the operation-scoped complement of every named exact
return, exception/message, and effect/state trace. It is generic behavior
closure, not a task runner or transport rule. Classify it on every reachable
row. Required and forbidden sets must be nonempty, disjoint, and together equal
the operation's entire observable alphabet.

Use `Effects` for exact observable reads, writes, calls, and outputs:

```text
read:input
write:selection="kept"; call:refresh; output:stdout="done\n"
```

If a relevant shape, state transition, ordering relation, or cross-call
invariant cannot be expressed by current strict syntax and canonical Semantic
IR, stop with `PROOF BLOCKED`. Prose outside the strict table is not a graded
property.

### Fill every strict column

Every operation table uses domain columns followed by:

1. `ID` — globally unique stable requirement or constraint ID.
2. `Operation` — exact operation ID.
3. `Reachability` — `reachable` or `excluded`.
4. `Required outcomes` — nonempty exact allowed set for reachable rows.
5. `Forbidden outcomes` — nonempty exact rejected set for reachable rows.
6. `Effects` — exact common effects, or `none`.
7. `Invariants` — typed bound invariant IDs, or `none`. Never invent a prose
   placeholder.
8. `Input witnesses` — canonical JSON witnesses in deterministic expanded-row
   order when required by current strict bindings; `none` for excluded rows.
9. `Enforced by` — `none` throughout phase 1.
10. `Evidence` — a span into the frozen instruction or the frozen reference.
    A bare span means the instruction; write `reference:94-101` to cite the
    solution. The instruction carries the contract, the reference carries the
    mechanism.
11. `Constraint reason` — required for excluded rows and `—` otherwise.

An instruction anchor proves where the author derived the row; it does not by
itself prove that the prose entails the row. The independent reviewer checks
that relationship using the phase-1 sources.

### Freeze before test access

Run structural lint until it reports no error:

```sh
hyperray spec-lint spec.md --instruction instruction.md --reference solution.patch --task-id <task-id>
```

`spec-lint` checks strict syntax, declared values, expansion, IDs,
reachability/constraint shape, and row partition structure. It does not inspect
code or tests and cannot establish reference or verifier semantics.

Before opening tests:

- verify every full-N-way assignment is represented once;
- verify every `Enforced by` cell is `none`;
- record the spec digest, instruction digest, reviewed source manifest,
  reviewer identity, disagreements, and decision in the authoring record; and
- preserve immutable pre-test bytes so later review can prove that only test
  mappings changed.

In audit shorthand: every Enforced by cell set to none before test access, and
only Enforced by cells changed after the public/hidden mapping pass.

Do not claim acceptance when a row, constraint, source anchor, observable
effect/state, or interaction remains uncertain.

## Phase 2: map and compare the real tests

Only after the semantic freeze, inspect both public and hidden tests, their
runner, setup/teardown, ordering, shared state, test command, and authoritative
pass signal. Record stable Test IR IDs in `Enforced by`; leave `none` when no
test actually observes a row. A name match is not enforcement evidence.

Treat the complete verifier as one global predicate over a full implementation
behavior `F : D -> O`. Preserve cross-case comparisons, chained operations,
shared fixtures, ordering, and the final pass calculation. Never reduce it to
the union of independent per-row accepted outcomes.

Review both disagreement directions:

```text
False-positive query:
  EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))

False-negative query:
  EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)
```

- A requirement behavior accepted by the verifier but outside `R` is a
  false-positive witness: an incorrect implementation can pass.
- A complete behavior inside `R` that the verifier rejects is a false-negative
  witness: tests impose an unstated or overly rigid restriction.

Also require exact reference acceptance `T(C) = true` separately from the
reference-correctness proof. Public and hidden tests are translated as they
exist; Hyperray never substitutes test logic derived from `spec.md`.

Compare final `spec.md` with the pre-test bytes. Only `Enforced by` cells may
change. If any domain, row, outcome, effect, state, constraint, or provenance
anchor needs revision, restart phase 1 and create a new semantic freeze.

## Formal and diagnostic boundaries

Only the exact Semantic IR checks over complete `D`, `O`, `R`, `C`, and `T`
can issue the mathematical all-clear:

```text
EXISTS x,o: C(x,o) AND NOT R(x,o)                          = UNSAT
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))                 = UNSAT
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)               = UNSAT
T(C)                                                        = true
```

Coverage, PICT, mutation testing, fuzzing, property-based testing, dependency
edge discovery, and differential samples may find useful witnesses. None can
produce `VERIFIED`, and none may fill a missing full-N-way row or semantic
translation.

## Final review

1. Instruction/issue, base, reference diff, relevant code/dependencies, and
   environment were reviewed before any test artifact.
2. Every operation has exact finite local domains and complete full-N-way
   reachable/excluded coverage.
3. Every reachable row closes exact outcomes, effects, and relevant state.
4. Every excluded row has a defensible impossible-case constraint.
5. Every row has stable ID and instruction/PR provenance.
6. Public and hidden tests were attached only after the semantic freeze.
7. Both false-positive and false-negative directions were reviewed against the
   real global verifier predicate.
8. Final semantic bytes differ from the pre-test freeze only in `Enforced by`.
9. Unsupported, partial, stale, sampled, or unknown evidence remains
   `PROOF BLOCKED`.
