# The evidence rule

Frozen. This decides where a `spec.md` row comes from. Everything else in
authoring follows from it.

## The rule

**The instruction gives the contract. The code gives the mechanism.**

- A row exists because the contract owes that behaviour.
- The code decides the row's exact shape: the conditions, the value lists, the
  exact outcomes.

Two consequences, and both matter:

- Code does something the contract never asked for → an implementation choice.
  It is not graded. Grading it rejects a correct solution written differently.
- Contract asks for something the code does not do → the reference is buggy.
  That is a finding, not a row to soften.

## Why the instruction alone is not enough

`instruction.md` is the prompt handed to the agent. Harbor's task-authoring
guide tells the author to write it this way:

> State the goal concretely — what behavior to produce.
> Specify expected outputs.
> Include constraints.
> **Don't leak the tests** — describe what "done" looks like, not how you'll
> check it.

So it hides the rubric, not the contract. The behaviour owed is stated; the way
it gets checked is withheld. An instruction that also omits the contract is a
defective task, because the agent cannot build what it was never told.

The prompt is prose, though. It says "emit it once at a legal dominating
point". It does not say which five conditions the pass evaluates, or that the
barrier set is exactly `can_trap`, `is_call`, `can_load`, `can_store`,
`other_side_effects`. Only the code says that, and a row needs it to be finite.

## Why the code alone is not enough

If every row is read off the reference, then "does the reference satisfy the
spec" is asking whether the code matches itself. That obligation dies, and all
that survives is "do the tests pin the code down".

Deriving from code alone also grades the author's incidental choices, which
rejects correct alternative solutions.

## Anchoring

The `Evidence` column carries the evidence for the row. A span may
name which artifact it points into:

```text
1              instruction.md line 1 (bare span, the default)
instruction:1  the same, written out
reference:94-101   solution.patch lines 94-101
reference:187-193; reference:226   two spans, semicolon separated
```

`ray spec-lint` takes `--reference <path>` alongside `--instruction <path>`. A
`reference:` anchor without `--reference` is rejected. A row anchored only into
the reference contributes no instruction clause, which is what lets it state a
mechanism detail the prose could not carry.

Anchoring a row into the reference does not make it graded. The contract test
comes first: a row with no contract backing is dropped, whatever the code does.

## Worked example

Task: eliminate redundant pure computation across CFG forks.

Contract, from the two-line prompt:

> Emit it **once** at a legal dominating point **where all operands are
> available** [...] If any successor does not need the result, **keep the
> computation path-local**.
> Do not move work **across a call, trap, or memory effect** [...] Existing
> **rematerialization** and **loop-invariant placement** behavior must remain
> intact.

Mechanism, from `solution.patch`: the hoist guard is one conjunction over five
conditions, and the barrier set is five opcode predicates.

Result: six rows covering all thirty-two assignments.

| row | contract backing |
|---|---|
| `REQ-hoist-once-to-fork` | "emit it once at a legal dominating point" |
| `REQ-operands-must-dominate-fork` | "where all operands are available" |
| `REQ-partial-demand-stays-local` | "keep the computation path-local" |
| `REQ-branch-barrier-blocks-hoist` | "do not move work across a call, trap, or memory effect" |
| `REQ-remat-takes-precedence` | "rematerialization ... must remain intact" |
| `REQ-no-fork-no-hoist` | structural: no fork, nothing to deduplicate |

The reference has a sixth condition — it skips a value already demanded locally
in the fork block. Nothing in the contract asks for it. It is an implementation
choice, so it is not a parameter of the table and not graded.

## Two failures this rule exists to prevent

Both were real, both in one sitting.

1. **Prose read as mechanism.** "Emit it once" was read as "copy it into each
   branch" when a barrier was present. Five rows were written on the guess. The
   code says a barrier lowers the insertion point and still emits one copy.
   Reading the code first prevents this.

2. **Code read as contract.** Every condition in the hoist guard was made a
   graded parameter, including one the prompt never asks for. A correct
   implementation without that guard would have been marked wrong. Applying the
   contract test prevents this.
