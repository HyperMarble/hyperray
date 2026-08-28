# Current strict spec examples

Use these patterns only after deriving behavior from the instruction/issue,
base, reference diff, and relevant environment without test access. The
complete maintained starting point is [the spec template](../templates/spec.md).

## Exact full-N-way interaction

This two-by-two operation keeps validation precedence explicit. Four rows are
required; separate one-dimensional rules would lose the interaction.

```markdown
Inputs: persist(target: string, payload: string).
Grounding: persist.target_state."writable" = when target == "writable"; witness {"payload":"valid","target":"writable"}.
Grounding: persist.target_state."read-only" = when target == "read-only"; witness {"payload":"valid","target":"read-only"}.
Grounding: persist.payload_kind."valid" = when payload == "valid"; witness {"payload":"valid","target":"writable"}.
Grounding: persist.payload_kind."invalid" = when payload == "invalid"; witness {"payload":"invalid","target":"writable"}.

Parameters: `target_state` ("writable" / "read-only"), `payload_kind` ("valid" / "invalid").

| target_state | payload_kind | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| "writable" | "valid" | REQ-persist-writable-valid | persist | reachable | return "stored" | raise ValidationError containing "invalid payload"; raise PermissionError containing "read-only target"; other outcome | write:target="stored" | none | [{"payload":"valid","target":"writable"}] | none | 1 | — |
| "writable" | "invalid" | REQ-persist-writable-invalid | persist | reachable | raise ValidationError containing "invalid payload" | return "stored"; raise PermissionError containing "read-only target"; other outcome | none | none | [{"payload":"invalid","target":"writable"}] | none | 1 | — |
| "read-only" | "valid" | REQ-persist-readonly-valid | persist | reachable | raise PermissionError containing "read-only target" | return "stored"; raise ValidationError containing "invalid payload"; other outcome | none | none | [{"payload":"valid","target":"read-only"}] | none | 1 | — |
| "read-only" | "invalid" | REQ-persist-readonly-invalid | persist | reachable | raise ValidationError containing "invalid payload" | return "stored"; raise PermissionError containing "read-only target"; other outcome | none | none | [{"payload":"invalid","target":"read-only"}] | none | 1 | — |
```

The last row deliberately chooses validation over permission rejection when
both clauses hold. That precedence must come from phase-1 evidence, not from
which assertion happens to appear in a test.

## Impossible combination

An excluded row still appears in the full product and states why it has no
path:

```markdown
| "closed" | "write" | CON-session-closed-write | session_request | excluded | — | — | — | — | none | none | — | the frozen API cannot issue write requests for a closed session |
```

“Unsupported,” “not tested,” and “the reference does not do this” are not
valid constraint reasons.

## Permitted alternatives

When the instruction permits two externally distinguishable results, keep both
in the required set so fairness analysis can detect an over-rigid test:

```markdown
| "ready" | REQ-drive-ready | drive | reachable | return "scheduled"; return "immediate" | raise StateError containing "not ready"; other outcome | none | none | [{"state":"ready"}] | none | 4-6 | — |
```

A hidden test that accepts only `"scheduled"` may pass the reference yet reject
the permitted `"immediate"` behavior. That is the false-negative direction;
do not narrow the row after reading the test.

## Slash-safe values

The exact separator is ` / ` outside JSON quotes:

```markdown
Parameters: `route` ("/api/v1" / "https://example/x/y?q=1/2" / "2026/08/27" / "left / right" / "snowman ☃").
```

Quoted `"any"` is a literal value. Unquoted `any` is a wildcard in a row.
Malformed quoting, invalid JSON escapes, mixed bare/quoted tokens, and
slash-bearing bare values fail closed.

## Observable state and effects

Use exact effect syntax rather than prose:

```text
read:input
write:selection="kept"; call:refresh; output:stdout="done\n"
return unit with call:engine.find, write:phase="Sending"
```

If a relevant type, shape, state transition, ordering rule, or bounded history
cannot be expressed in the strict format and Semantic IR, keep the task
`PROOF BLOCKED` until the generic language supports it.

## Relational requirements

Every real prompt carries at least one requirement that relates two runs
rather than describing one:

> *"applying the same operation or pipeline again to its output must produce
> no further changes"* · *"existing rematerialization and loop-invariant
> placement behavior must remain intact"* · *"preserve existing semantics"*

A row states one outcome for one input, so these have no direct form.
Express the relation as an operation that computes it and returns a bool.
`Inputs:` accepts any operation the frontend can translate, so the relation
becomes an ordinary bounded row with a witness like any other.

```markdown
Inputs: holds(aspect: string).
Grounding: holds.aspect."idempotent" = when aspect == "idempotent"; witness {"aspect":"idempotent"}.
Grounding: holds.aspect."behaviour-preserving" = when aspect == "behaviour-preserving"; witness {"aspect":"behaviour-preserving"}.

Parameters: `aspect` ("idempotent" / "behaviour-preserving").

| aspect | ID | Operation | Reachability | Required outcomes | Forbidden outcomes | Effects | Invariants | Input witnesses | Enforced by | Evidence | Constraint reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| "idempotent" | REQ-reapplication-is-fixpoint | holds | reachable | return true | return false; other outcome | none | none | [{"aspect":"idempotent"}] | none | 2 | — |
| "behaviour-preserving" | REQ-output-behaviour-preserving | holds | reachable | return true | return false; other outcome | none | none | [{"aspect":"behaviour-preserving"}] | none | 2 | — |
```

Collect them under one `holds(aspect)` operation, one value per property.
The predicate has to be computable by the frontend, because the proof
evaluates it — prose in `Required outcomes` is rejected, correctly.

## Non-scalar observables

When the real observable is not a value the operation returns — where an
instruction was placed, whether a connection was replaced, which credential
was resolved — name the decision as an operation and return its label.

```markdown
Inputs: placement_block(purity: string, demand: string, barrier: string).

| "pure" | "all-successors" | "absent" | REQ-place-hoist-to-fork | placement_block | reachable | return "fork" | return "each-successor"; return "unchanged"; other outcome | ... |
```

Between these two patterns and ordinary scalar rows, six real accepted
tasks across Go, Rust, and C++ compiled with no `unsupported` construct.

## Evidence anchors into the reference

A row whose exact shape comes from the reference solution cites it directly.
A bare span still means the instruction.

```
| "non-numeric" | REQ-parse-limit-non-numeric | parse_token | reachable | return "error" | return "term"; return "filter"; other outcome | none | none | [{"key":"limit","value_kind":"non-numeric"}] | none | reference:95-99 | — |
```

Lint with both artifacts:

```sh
ray spec-lint spec.md --instruction instruction.md --reference solution.patch --task-id <task-id>
```

The instruction carries the contract; the reference carries the mechanism.
The frozen statement is [the evidence rule](../../../docs/specs/evidence-rule.md).
