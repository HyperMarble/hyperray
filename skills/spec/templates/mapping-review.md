# Public and hidden test comparison

Complete this review only after the semantic rows and pre-test spec digest are
frozen. Tests are enforcement evidence and one independently translated global
predicate; they do not define requirements.

## Frozen inputs

| Field | Value |
|---|---|
| Task or PR ID | |
| Pre-test spec path and SHA-256 | |
| Final spec path and SHA-256 | |
| Frozen instruction/issue path and SHA-256 | |
| Public test artifact paths and SHA-256 values | |
| Hidden test artifact paths and SHA-256 values | |
| Test runner/tool identity | |
| Exact verifier command | |
| Authoritative pass signal | |

## Requirement-to-test mapping

Map actual assertions and observations, not names or shared words.

| Requirement ID | Final `Enforced by` Test IR IDs or `none` | Public/hidden | Exact observed return/effect/state | Setup/order/shared-state dependencies | Confidence or reason for `none` |
|---|---|---|---|---|---|
| | | | | | |

## Test-only restriction inventory

Every assertion that affects pass/fail must be permitted by at least one
frozen semantic row, including type, shape, label, ordering, exception text,
internal-call, timing, and state restrictions.

| Test IR ID | Exact restriction | Permitted requirement IDs | Instruction/issue provenance | Unstated or overly rigid? | Resolution |
|---|---|---|---|---|---|
| | | | | `yes` or `no` | |

## Global verifier predicate review

| Check | Result/evidence |
|---|---|
| Every public and hidden test affecting grading is translated | |
| Setup, teardown, ordering, shared state, and chained calls are preserved | |
| Cross-case comparisons remain global rather than per-row unions | |
| Exact command and authoritative pass signal are modeled | |
| Test names, coverage, or scores were not used as semantic authority | |

## False-positive direction

Review whether an incorrect complete behavior can pass:

```text
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))
```

| Finding/witness ID | Passing behavior outside `R` | Missing public/hidden enforcement | Real verifier confirmation | Resolution |
|---|---|---|---|---|
| | | | | |

## False-negative direction

Review whether public or hidden tests reject behavior permitted by the frozen
requirements:

```text
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)
```

| Finding/witness ID | Permitted complete behavior rejected by `T` | Unstated/rigid restriction | Real verifier confirmation | Resolution |
|---|---|---|---|---|
| | | | | |

## Exact reference acceptance

| Check | Result/evidence |
|---|---|
| Reference correctness query is `UNSAT` | |
| Exact frozen verifier accepts exact reference: `T(C) = true` | |
| Reference run and bounded correctness were treated as separate results | |

## Phase-boundary review

| Check | Result |
|---|---|
| Pre-test and final specs compile against identical instruction bytes | `pass` or `fail` |
| Pre-test `Enforced by` cells are all `none` | `pass` or `fail` |
| Only final `Enforced by` cells changed | `pass` or `fail` |
| Requirement/test disagreements remain visible | `pass` or `fail` |
| Reviewer identity | |
| Review completed at (UTC) | |

If tests reveal an incorrect requirement, restart pre-test authoring and create
a new semantic freeze. Do not rewrite an existing freeze to match the tests.
