# spec.md API schema

The exact input contract `hyperray spec-lint` accepts. Anything outside it is
rejected at ingestion; the compiler never guesses or repairs. Extracted
from `internal/speccompiler` and `internal/specparser`, not from prose.

Invocation:

```sh
hyperray spec-lint spec.md --instruction instruction.md --task-id <id>
```

Success prints `spec: complete` with an IR digest and a frozen-semantics
digest. Any diagnostic blocks compilation.

## 1. Document directives

Each operation section declares its signature, then one grounding per
declared domain value, then the parameter domains.

### Inputs

```
Inputs: <operation>(<name>: <type>, ...).
```

Types: `bool`, `integer`, `string`. The trailing period is required. Every
input named here must be assigned exactly once by every witness.

### Scope, Classify, Observe

The bridges from the finite model to the real system
(`docs/specs/proof-requirements.md`, groups B and C). Required for every
operation that names a string outcome label; an operation whose outcomes are
all concrete values (booleans, numbers, unit, exceptions) may omit them until
the proof runner lands.

```
Scope: <operation> = <the finite real input set this operation covers>.
Classify: <operation> = command <executable mapping a real input to its row>.
Observe: <operation>."<label>" = command <executable deciding the label on real output>.
```

`Scope` names the finite real inputs the rows partition. `Classify` names the
executable answering "which row is this real input?" — total and
deterministic over the scope. `Observe` names the executable deciding one
string label on real output; every string label in the operation's Required
or Forbidden outcomes needs exactly one. Each may be declared once per
operation (per label for Observe); the trailing period is required. A label
without an observer is a word, and a proof over words does not transfer to
the real system.

### Grounding

```
Grounding: <operation>.<domain>."<value>" = when <expression>; witness {<canonical JSON>}.
```

One per declared value. `when` is the membership predicate that defines
which concrete inputs belong to that semantic value; `witness` is one
concrete assignment that satisfies it. The witness is what turns a category
name into a finite proof point — a domain of names alone is not a finite
input universe.

Expression grammar: `==`, `!=`, `and`, `or`, `not`, parentheses, the
literals `true`, `false`, `null`, integers, quoted strings, input names, and
integer arithmetic.

### Universe

```
Universe: <operation>.<input> = values [<compact canonical JSON array>].
```

The complete finite set of concrete runtime values that input admits. One
per input, values matching the input's declared type, distinct, compact
canonical JSON, trailing period.

```
Universe: persist.target = values ["writable","read-only"].
Universe: persist.payload = values ["valid","invalid"].
```

**`spec-lint` does not require this; the proof does.** A spec without it
compiles and then fails downstream with `input "x" has no explicit finite
Universe`, because concrete quantification has nothing to range over.

This is what makes the bound *proved* rather than asserted. `Grounding`
gives one witness per semantic category; `Universe` states that those
categories exhaust the input. A domain of category names without a Universe
is a finite list of labels over an unbounded input — the exact gap that
lets a proof verify a smaller problem than the real one.

### Parameters

```
Parameters: `<domain>` ("<v1>" / "<v2>"), `<domain2>` ("<v3>" / "<v4>").
```

The separator is exactly ` / ` **outside** JSON quotes, so a value may
contain slashes: `"/api/v1"`, `"https://example/x?q=1/2"`, `"2026/08/27"`.
Quoted `"any"` is a literal value; bare `any` is the row wildcard. Bare
slash-bearing values, mixed bare/quoted tokens, and invalid escapes fail
closed.

## 2. Table columns

Domain columns first, in `Parameters:` order, then these eleven, in order:

| # | Column | Contents |
|---|---|---|
| 1 | `ID` | unique, matches `^[A-Za-z_][A-Za-z0-9_.:#-]*$` |
| 2 | `Operation` | the operation ID from `Inputs:` |
| 3 | `Reachability` | `reachable` or `excluded` |
| 4 | `Required outcomes` | nonempty outcome set; `—` for excluded |
| 5 | `Forbidden outcomes` | nonempty outcome set; `—` for excluded |
| 6 | `Effects` | effect set or `none` |
| 7 | `Invariants` | typed invariant IDs or `none` — never prose |
| 8 | `Input witnesses` | compact canonical JSON array, expanded-row order (§2.1); `none` for excluded |
| 9 | `Enforced by` | test IDs, or `none` (always `none` in phase 1) |
| 10 | `Evidence` | span into the frozen instruction or reference; `—` for excluded |
| 11 | `Constraint reason` | required for excluded rows; `—` otherwise |

Every full N-way assignment must appear exactly once across the rows.

### 2.1 Input witness form and order

**Compact and canonical.** No space after a comma, object keys in lexical
order. `[{"a":1}, {"b":2}]` is rejected; `[{"a":1},{"b":2}]` is accepted.

**Expanded-row order: the LAST declared parameter varies fastest.** For
`Parameters: shape (A / B), invocation (X / Y)` a row covering both
parameters lists its witnesses as

```
A,X   A,Y   B,X   B,Y
```

Getting this backwards produces
`unreachable: element N does not satisfy expanded assignment`. Generate the
array rather than typing it:

```python
[{"invocation": i, "shape": s} for s in shapes for i in invocations]
```

## 3. Outcome grammar

An outcome cell is a `;`-separated set. Each element is one of:

| Form | Meaning |
|---|---|
| `success` | completes, value unconstrained |
| `timeout` | does not complete — no effects allowed |
| `other outcome` | the complement of everything named — no effects allowed |
| `return` / `return unit` | returns the unit value |
| `return <literal>` | returns exactly that literal |
| `raise <Type> containing "<message>"` | raises that type carrying that text |

Literals: `null`, `true`, `false`, an integer, or a quoted string. Nothing
else — a bare word is prose and is rejected.

Any form except `timeout` and `other outcome` may carry effects:

```
return "stored" with write:target="stored", call:refresh
```

`Forbidden outcomes` must enumerate the alternatives, and `other outcome`
closes the set. A row naming only its own required value leaves the rest of
the outcome space unclassified and is rejected as incomplete.

## 4. Effect grammar

```
<kind>:<target>
<kind>:<target>=<literal>
```

Kinds: `read`, `write`, `call`, `output`. Multiple effects are separated by
` / ` in the `Effects` column, or by `,` after ` with ` in an outcome.
Duplicates are rejected.

```
read:input
write:selection="kept" / call:refresh / output:stdout="done\n"
```

## 5. Evidence

Where the row came from. An anchor is a span, optionally naming which frozen
artifact it points into:

```
1                     instruction line 1 (bare span, artifact defaults to instruction)
instruction:1:474-1:650   the same artifact, written out, with columns
reference:94-101          solution.patch lines 94-101
reference:187-193; reference:226   two spans, separated by `;`
```

Span forms are `line`, `line:column`, `start-end`, and
`start:column-end:column`. A span must lie inside that artifact's real line
and column count. `reference:` requires `hyperray spec-lint --reference <path>`;
without it the row is rejected.

Two artifacts exist because they carry different things.

| artifact | carries |
|---|---|
| instruction | the **contract** — what behaviour is owed |
| reference | the **mechanism** — exact conditions, value lists, outcomes |

A benchmark instruction is a problem statement, not a specification. Harbor's
task-authoring guide tells the author to hide the rubric — "describe what
done looks like, not how you'll check it" — so the prose states the contract
and withholds the checking. It will not name the five conditions a pass
evaluates, or that a barrier set is exactly `can_trap`, `is_call`,
`can_load`, `can_store`, `other_side_effects`. Only the reference does, and a
row needs that to be finite.

Every **reachable** row needs at least one anchor. Anchoring into the
reference does not by itself make a row graded: the contract decides that. A
condition the reference implements that the contract never owes is an
implementation choice — leave it out of the table, because grading it rejects
a correct solution written differently. A behaviour the contract owes that the
reference omits is a reference bug, not a row to soften.

An anchor records where the author derived the row. It does not by itself
prove the source entails the row; that is the reviewer's judgement.

The frozen statement of this is [the evidence rule](../../../docs/specs/evidence-rule.md).

## 6. Prose outside tables

Section text may describe and orient. It may not carry a requirement.
Any line outside a strict table containing `must`, `shall`, `required`,
`forbidden`, `never`, or `always` is rejected as
`prose-only-requirement`, naming the line.

```
rejected:  Caller cancellation must not cancel those retries.
accepted:  The rows below record which races leave a retry loop running.
```

The rule keeps every graded requirement in exactly one place — a row — so
nothing is enforced from narrative the compiler cannot expand.

## 7. Relational requirements

An implementation is modelled as one outcome per input case, so a
requirement relating two runs — idempotence, preservation against the base,
determinism — has no direct form. Every real prompt carries at least one:

> *"applying the same operation or pipeline again to its output must produce
> no further changes"*
> *"existing rematerialization and loop-invariant placement behavior must
> remain intact"*
> *"preserve existing semantics"*

Express the relation as an operation that computes it and returns a bool.
`Inputs:` accepts any operation the frontend can translate, so the relation
becomes an ordinary bounded row with a witness like any other.

| requirement | operation | required outcome |
|---|---|---|
| idempotent | `is_fixpoint(module: string)` | `return true` |
| preserves base behaviour | `matches_base(input: string)` | `return true` |
| deterministic across runs | `runs_agree(input: string)` | `return true` |
| output still valid | `is_valid(output: string)` | `return true` |

```
Inputs: is_fixpoint(module: string).
Grounding: is_fixpoint.shape."deep-linear" = when module == "deep-linear"; witness {"module":"deep-linear"}.

| shape | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | ... |
| "deep-linear" | REQ-opt-idempotent-deep-linear | is_fixpoint | reachable | return true | return false; other outcome | ... |
```

Do not smuggle the relation into prose in `Required outcomes` — that is
rejected as `prose-only-requirement`, correctly. The relation has to be
computable by the frontend, because the proof has to evaluate it.

## 8. What the compiler rejects

| Diagnostic | Cause |
|---|---|
| `invalid-input` | malformed cell, witness not assigning every input exactly once |
| `prose-only-requirement` | an outcome or invariant written as prose |
| `non-finite-domain` | a domain that is not an explicit finite value list |
| `missing-bridge` | an operation with string outcome labels lacks a Scope or Classify declaration, or a string label lacks its Observe declaration |
| `quantifier-as-label` | warning: a lone true/false row anchors into contract text that quantifies (every / any / all / regardless); a quantified variable is a domain, so declare a `Universe:` ranging over it instead of one boolean aspect |
| `duplicate-id` | repeated row ID, or a domain value repeated after splitting |
| `incomplete` | a row not classifying an outcome in the operation's space |
| `overlapping` | two rows covering the same assignment |
| `unreachable` | a row matching no finite assignment, or a witness failing its own membership expression |
| `invalid-provenance` | a span outside the frozen instruction |
| `stale-artifact` | the instruction bytes do not match the recorded digest |
| `invalid-reference` | a row naming an operation, domain, or ID that was never declared |
| `missing-domain` | a table column with no matching `Parameters:` domain |
| `unsupported-construct` | a construct the strict format cannot express |

`unsupported` is not a failure to route around. It means the task stays
`PROOF BLOCKED` until the generic format can express the behaviour.
