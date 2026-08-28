# Pre-test semantic authoring record

Complete this record before opening public or hidden tests. It records human
review of the finite behavior authored in `spec.md`; it is not a substitute for
strict compilation or later formal evidence.

## Identity and freeze

| Field | Value |
|---|---|
| Task or PR ID | |
| Author identity | |
| Independent reviewer identity | |
| Review decision | `accepted`, `rejected`, or `blocked` |
| Review completed at (UTC) | |
| Test access during authoring | `not accessed` |
| Frozen instruction/issue path and SHA-256 | |
| Frozen pre-test spec path and SHA-256 | |
| Strict lint command and result | |

## Phase-1 source manifest

List only sources available before test access.

| Role | Artifact path/ref | Commit or SHA-256 | Exact spans inspected | What it establishes |
|---|---|---|---|---|
| instruction, issue, or PR description | | | | intended behavior and provenance lines |
| base source and relevant callers | | | | existing branches, state, and preserved behavior |
| reference solution or PR diff | | | | changed branches, returns, effects, and interactions |
| applied reference source | | | | complete post-change operation behavior |
| pinned dependency/environment evidence | | | | reachability, values, types, shapes, and bounds |

Confirm that no test file, test patch, test name, grader script, pass log, or
test-derived artifact was read directly or through another agent.

## Operation scope

| Operation ID | Entry point | Inputs/pre-state/history in scope | Observable boundary | Relevant callers/dependencies | Source evidence | Decision |
|---|---|---|---|---|---|---|
| | | | returns, exceptions, values, types, shapes, effects, and state | | | `accepted`, `rejected`, or `blocked` |

## Exact finite domains

| Operation ID | Domain ID | Exact declared values in order | Meaning and disjoint boundary of each value | Phase-1 evidence | Decision |
|---|---|---|---|---|---|
| | | | | | `accepted`, `rejected`, or `blocked` |

If typed `Inputs:` and `Grounding:` declarations are used, copy their exact
membership expressions and canonical witnesses here. Explain why the declared
semantic domains cover the fixed bounded input/state scope without using test
arguments as evidence.

## Impossible-case constraints

| Constraint ID | Operation-local full assignment | Exact reason the case is impossible | Phase-1 source evidence | Decision |
|---|---|---|---|---|
| | | | | `accepted`, `rejected`, or `blocked` |

Unsupported, inconvenient, or untested is not an impossible-case reason.

## Full-N-way row review

Review the deterministic expansion of every operation-local Cartesian product.

| Requirement ID | Full assignment | Required outcomes | Forbidden outcomes | Effects/state/order | Instruction/issue lines | Base/diff/environment evidence | Decision |
|---|---|---|---|---|---|---|---|
| | | | | | | | `accepted`, `rejected`, or `blocked` |

Confirm:

- every local assignment appears exactly once as reachable or excluded;
- simultaneous conditions and precedence were reviewed together;
- every reachable row closes the operation outcome alphabet;
- types, shapes, labels, calls, outputs, mutations, and state transitions are
  explicit when observable;
- every excluded row has a defensible constraint; and
- every `Enforced by` cell is `none`.

## Cross-clause and state interactions

| Interaction ID | Clauses/rows involved | Ordering, shared state, repeated-call, or precedence relation | Required representation | Phase-1 evidence | Decision |
|---|---|---|---|---|---|
| | | | | | `accepted`, `rejected`, or `blocked` |

## Source disagreements

| ID | Sources in disagreement | Exact disagreement | Resolution or blocker | Evidence | Decision |
|---|---|---|---|---|---|
| | | | | | `accepted`, `rejected`, or `blocked` |

The reference diff does not override the instruction/issue silently. Resolve a
real mismatch with the task owner or keep the task blocked.

## Freeze decision

| Check | Result |
|---|---|
| Strict compiler accepted the spec | `pass` or `fail` |
| Complete full-N-way row count matches domain products minus constraints | `pass` or `fail` |
| All reachable outcomes/effects/state are exact | `pass` or `fail` |
| All rows have instruction/issue provenance | `pass` or `fail` |
| All phase-1 source disagreements are resolved | `pass` or `fail` |
| All `Enforced by` cells are `none` | `pass` or `fail` |
| Frozen pre-test bytes and SHA-256 recorded before test access | `pass` or `fail` |
| Final decision | `accepted`, `rejected`, or `blocked` |
