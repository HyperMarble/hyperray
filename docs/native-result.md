# Native search result validation

## Contract

`execution/nativecheck.Run` operates the generated search and validates its report.
The caller supplies the execution request, replay executable, and inclusive input interval.
The caller must use artifacts from the same trusted preparation.
This API does not accept a caller-written search result as execution evidence.

The result preserves the search output and measured resource status.
Each memory measurement covers an observer worker interval, not subsequent report parsing.
It distinguishes `search_reported_complete`, `counterexample_reproduced`, and `incomplete`.
No outcome grants a proof capability or returns `PROVED`.

A completion report requires successful execution, full-state search, enabled assertions,
one zero-error summary, and one observation count equal to the declared interval size.
Overflow, partial-search diagnostics, absent records, and duplicate records cannot pass.
The full `u64` interval cannot pass a 64-bit observation counter without overflow.
Unknown or inconsistent report formats return errors with the execution result retained.

A counterexample must name an input inside the interval and one output.
The API repeats that input through the supplied replay executable.
Replay must return exit code 1, one observation, and the same input and output.
A stopped replay remains incomplete. A mismatch returns an error, not a reproduced bug.
Resource limits always take precedence over report text.

## Trust boundary

This validator reads native checker diagnostics, not a mathematical proof certificate.
It does not establish binary identity, source equivalence, or semantic coverage.
The native deterministic and trusted-execution assumptions remain necessary.
Subject output is not an authenticated channel. This API cannot resist forged checker output.
Observation counts are consistency evidence, not independent coverage evidence.
Arbitrary signatures, thread instrumentation, and formal result validation remain separate work.

The report grammar comes from the generated SPIN 6.5.2 `wrapup` function
and `execution/native/observe.c` and `replay.c`.
The acceptance tests must include all existing native and Cargo preparation cases.
Negative tests must include incomplete searches, malformed reports, replay mismatches,
invalid bounds, and every observer resource status.

## Measured result: 2026-09-05

All seventeen native cases and four Cargo cases passed through the public result API.
The incorrect cases triggered automatic replay. The depth-limited case remained incomplete.
The real replay rejection test retained search output for a different replay executable,
a missing replay executable, and an incorrect input interval.

Nine decision and public request tests passed. Go regression tests, static analysis,
and race tests passed. The source review covered fourteen files.
The largest file had 72 lines. The longest function had 35 lines.
The Linus-style skill set the source limits. The completion-gate skill required the full matrix.

The retained log is `/tmp/hyperray-native-result.Em47Ah/matrix.log`.
The Cargo cases measured 11,862,016 bytes at their largest worker-plus-observer peak.
The earlier native cases measured 35,536,896 bytes at their largest corresponding peak.
These figures exclude compilation and do not establish a hard memory cap.

This milestone closes diagnostic result validation, not formal proof acceptance.
The full product goal remains open.
