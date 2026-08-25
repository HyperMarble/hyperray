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
- Write the condition table: every row is one combination mapped to one
  required behavior.
- Run `ray spec-lint` immediately. It fails on two things only:
  a combination with no row (completeness), or two rows that conflict
  (disjointness).
- Where instruction.md is silent on a combination spec-lint flags,
  resolve it and write the resolved behavior directly into the table —
  don't guess and move on.

Full worked example: `examples/fhplex-task/spec.md`. One clause from it:

```markdown
## 1. Construction

| n_components | component_kind | column_labels | Required behavior |
|---|---|---|---|
| 0 | — | — | raise ValueError containing "at least one component" |
| 1+ | scalar | — | raise ValueError containing "labelled rows and columns" |
| 2+ | labelled array distribution | differ in value or order | raise ValueError containing "identical columns" |
| 1+ | labelled array distribution | identical in value and order | construct successfully; combined row index is the concatenation of each component's index, in component order |
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
