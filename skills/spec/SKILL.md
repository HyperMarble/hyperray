---
name: spec
description: Write spec.md scoped to an existing task's instruction.md —
  not a new task, the condition-table contract for one that already
  exists. Use when a task has instruction.md but no spec.md yet.
---

Don't derive the whole spec from instruction.md in one pass. Answer
these four questions first, then write one condition table at a time,
running `ray spec-lint` after each one.

1. **Goal, concretely.** One sentence — what's actually being asked.
2. **Expected outputs.** The specific objects/functions in scope.
3. **Constraints.** What's explicitly excluded — existing code not to
   touch, out-of-scope behavior. Check exclusions against the actual
   base commit, don't assume.
4. **Don't leak the tests, in reverse.** Every row states required
   *behavior*, never a literal assertion — `ray coverage` derives test
   cases from spec.md, not the other way around.

For each clause of instruction.md:

- Name the parameters it varies on — this is the actual judgment call,
  not the table syntax.
- Declare each parameter's domain in a `Parameters:` line immediately
  above the table: `` `name` (v1 / v2 / v3) ``, `/`-separated. This is
  the only place values get declared — a table cell can never introduce
  a value the `Parameters:` line didn't list.
- Write the condition table: every row is one combination mapped to one
  required behavior. A cell holds exactly one of:
  - a single declared value
  - a `/`-separated list of declared values (a compound row covering
    each one — same separator as the `Parameters:` line, so a value's
    own name is free to contain a comma)
  - `any` — every declared value of that column applies equally
  - `—` — the column does not apply to this row at all
- Run `ray spec-lint` immediately. It fails on three things: a
  combination with no row (completeness), two rows that conflict
  (disjointness), or a cell value the `Parameters:` line never declared
  (undeclared-value).
- Where instruction.md is silent on a combination spec-lint flags,
  resolve it and write the resolved behavior directly into the table —
  don't guess and move on.

Full worked example: `examples/fhplex-task/spec.md`. One clause from it:

```markdown
Parameters: `n_components` (0 / 1 / 2+), `component_kind` (labelled
array distribution / scalar / non-distribution object), `column_labels`
(identical / differ).

| n_components | component_kind | column_labels | Required behavior |
|---|---|---|---|
| 0 | — | — | raise ValueError containing "at least one component" |
| 1 / 2+ | scalar | — | raise ValueError containing "labelled rows and columns" |
| 1 / 2+ | non-distribution object | — | raise TypeError containing "probability distributions" |
| 1 | labelled array distribution | — | construct successfully; combined row index is that one component's index |
| 2+ | labelled array distribution | differ | raise ValueError containing "identical columns" |
| 2+ | labelled array distribution | identical | construct successfully; combined row index is the concatenation of each component's index, in component order |
```

## Common pitfalls

- Deriving all of spec.md from instruction.md in one pass instead of
  clause-by-clause with spec-lint in the loop — produces
  plausible-looking tables with hidden gaps. A wrong row in a clean
  table is harder to catch than a wrong sentence in prose, not easier.
- Writing scope/decision narrative into spec.md itself — keep it to
  scope + tables. This skill is where the process guidance belongs, not
  the shipped file.
- A table row that restates a test assertion instead of required
  behavior — leaks the tests in reverse.
- Touching code outside what question 3 named as in scope, even when it
  looks similar to in-scope code. Check against the base commit before
  assuming something is fair game.
- Silently assuming behavior for a combination instruction.md never
  addressed, instead of writing the resolved decision into the table.
- `any` and `—` are reserved keywords, never data values. If the real
  behavior needs a value that happens to be the word "any" (e.g. a
  direction that's "up, down, or any"), name it precisely instead —
  `unrestricted`, `no-preference`, whatever the actual behavior means —
  never the bare word `any`. A real task hit exactly this: "any" as a
  literal value collided with the wildcard keyword and produced a
  disjointness conflict.
