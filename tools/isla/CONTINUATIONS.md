# Native Isla continuations v1

This artifact is a separate cumulative patch. It starts at upstream Isla commit
`7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6` and includes the accepted execution
guards plus the bounded continuation changes. It does not change the Go
initial-thread code, Rust compiler adapter, research checkout, or accepted
rebuilt binaries.

## Source and artifacts

- Source: `/Users/hak/isla-continuation-v1-source`
- Upstream: `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`
- Internal Cargo target: `/Users/hak/isla-continuation-v1-target`
- Evidence logs: `/Users/hak/isla-continuation-v1-evidence`
- Patch: `tools/isla/patches/continuation-v1.patch`
- Patch SHA-256: `291e4fd54374bcad79be5a0923b31ab679b03cc9a16017281ae1ed94b038b59f`
- Existing guard patch SHA-256: `fd5c8b12d4cb45b1405ed3f8a814c5de06eb39eb5c7aad11938d5eec10bf6af8`

The cumulative patch passed `git apply --check` against a clean archive of the
pinned upstream commit. No commit or push was made.

## Native contract implemented

`isla_lib::executor::continuation` provides an explicit initial
`ExecutionStart` containing a `LocalFrame` and paired `Checkpoint`, an owned
`ExecutionSession`, an opt-in `AfterAnnouncement` policy, cumulative timeout
limits, opaque same-session `SuspendedExecution` values, and an
`ExecutionBatch` containing every returned outcome. A batch status distinguishes
advanced, rejected, partially rejected, and no-work calls.

The pause is after the existing zero-announcement and PC-limit guards, the
instruction event, Unit assignment, and IR PC increment. It is before the
announced instruction executes. Resume restores the frozen frame and checkpoint
without `new_call` or a fresh `TaskId`. Mixed-session inputs return the foreign
continuation in the structured `Rejected` outcome and still advance valid
same-session tokens.

Checkpoint cycles are captured and restored explicitly. Solver queries refresh
Z3's `timeout` parameter from one absolute session deadline before both
`check_sat` and `check_sat_with`. No solver, AST, or model pointer is stored in
or returned by a continuation.

## Validation

Commands used, with `CARGO_BUILD_JOBS=2`, offline mode, and the internal target:

```text
CARGO_BUILD_JOBS=2 CARGO_TARGET_DIR=/Users/hak/isla-continuation-v1-target cargo test --manifest-path isla-lib/Cargo.toml continuation --offline
CARGO_BUILD_JOBS=2 CARGO_TARGET_DIR=/Users/hak/isla-continuation-v1-target cargo test --manifest-path isla-lib/Cargo.toml --test continuation_public --offline
CARGO_BUILD_JOBS=2 CARGO_TARGET_DIR=/Users/hak/isla-continuation-v1-target cargo test --manifest-path isla-lib/Cargo.toml --offline
```

Results:

- Focused continuation run: 8 passed. Log: `isla-continuation-v1-evidence/focused-test.log`.
- External public API run: 1 passed. Log: `isla-continuation-v1-evidence/public-api-test.log`.
- Full `isla-lib` run: 108 unit tests, 1 public integration test, and 3 doctests passed. Log: `isla-continuation-v1-evidence/isla-lib-test.log`.
- Focused coverage includes cycles, initial symbolic checkpoint state, repeated pauses, nested call closures, multiple forks, zero-announcement precedence, no-work status, exact cross-session rejection, mixed-token recovery, and cumulative zero-timeout failure.

## Acceptance boundary and unresolved cases

This evidence does not claim full Rust coverage. The required external public
run using the pinned real RV64 Sail/model configuration and a compiler-linked
ELF was not executed because no permitted Sail model and linked ELF inputs were
available in the allowed local source and tool paths. The required interrupt
once, invalid symbolic interrupt, PC-limit loop, sequential-memory callback
comparison, symbolic-renaming comparison, repeated real ELF resume comparison,
and deterministic SMT-unknown acceptance cases remain open and must be run
before claiming the full plan acceptance. Existing binaries were not replaced.

The implementation is same-process and borrowing only. It makes no persistent
snapshot, EEI, scheduler, or full-coverage claim.
