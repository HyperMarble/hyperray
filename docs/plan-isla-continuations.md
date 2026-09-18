# Plan: native Isla continuations

Status: implementation contract for one runtime prerequisite. Full Rust coverage remains open.

## Source evidence and purpose

The pinned Isla executor already has frozen `Frame` snapshots and solver `Checkpoint` values. `executor/frame.rs:107-175` retains registers, memory, call-stack closures, assumptions, visit counts, and taken interrupts. `Run::Suspended` exists but has no inspected producer. Existing whole-query trace collectors reject or ignore that outcome.

This stage makes existing ISA execution resumable through a public, same-process native API. It does not add instruction meanings, Linux syscall semantics, software scheduling, or persistent snapshots.

## Source and ownership

Astra plans and reviews. Luna high writes native code and tests. Preserve upstream licenses. Use an isolated source directory derived from upstream commit `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6` plus the accepted execution-guards patch. Never modify the dirty research checkout or accepted rebuilt binaries.

Preserve `tools/isla/patches/execution-guards-v1.patch` and its recorded hash. Publish a separate cumulative `tools/isla/patches/continuation-v1.patch` against the same upstream commit. Record the new version, source and binary identities, and acceptance evidence separately.

Native changes belong in executor suspension/collector code, task policy, and solver checkpoint code. Change frame encapsulation only when the public API needs it. Do not alter the Go initial-thread semantics.

## Exact pause boundary

An explicit opt-in policy pauses after `INSTR_ANNOUNCE` completes, before execution of the announced instruction. This is not instruction retirement.

Retain the existing zero-announcement and PC-limit guards. Emit the instruction event, assign the Unit result, and advance the saved IR PC before suspension. Then return `Run::Suspended` through a dedicated collector. Resume must not repeat the event, assignment, or visit count.

Do not use `StopAction::Kill` or `Abstract` as suspension. Those actions terminate or replace behavior. Default whole-query runners and their stop behavior remain unchanged.

## Public continuation contract

Expose an initial task, borrowed shared state, explicit pause policy, and cumulative session limits through a concrete public session API. Every public signature type must be externally constructible. Use one solver worker initially.

Each advance returns all suspended continuations and distinct finished, exit, dead-path, and error outcomes. The collector must not omit any outcome or queued fork. A suspended result is not a proof verdict.

Each opaque continuation owns a paired frozen frame and solver checkpoint. It retains the same TaskId, TaskState, stop policy, and cumulative deadline. Capture the pair before the solver context expires. Resume at the saved IR PC with the saved call-stack closure. Never call `LocalFrame::new_call` or allocate a fresh TaskId for resume.

Restore SMT assertions and next-variable identity into a fresh solver context. Apply each deferred fork condition once. Never export native solver, AST, or model pointers. Keep the borrowed IR, shared state, and task configuration alive.

## Cycle and memory invariants

Current solver checkpoints omit the cycle counter. Add explicit capture and restoration exactly once. A fresh solver starts at zero and replay does not restore `Event::Cycle`. Trace restoration alone is insufficient.

Acceptance covers the existing sequential-memory callback. Its current SMT-array symbol must remain paired with the corresponding checkpoint. Arbitrary callback external or interior state is not a deep-copy guarantee. These are same-process continuations, not disk-serializable snapshots.

Repeated pauses must not reset visit bounds, elapsed-time bounds, or other session resource budgets. An exhausted budget or SMT-unknown result remains a named failure, never successful truncated execution.

## Acceptance

First add focused native tests for checkpoint cycles, repeated pauses, nested calls, forks, and guard precedence. Then use an external native caller with the pinned Sail RV64 model, configuration, and a real compiler-linked ELF.

From identical initial inputs, compare uninterrupted execution with repeated announcement pause/resume. Compare every path's terminal or error status, ordered events, architectural state, and sequential-memory constraints. Handle symbolic-variable renaming explicitly. Do not compare only path counts or one final register.

Required cases include nested Sail calls, multiple symbolic forks, writes before and reads after pauses, and a nonzero cycle count before capture followed by further increments. Every feasible branch must remain represented.

Required negative cases include one interrupt firing once across resumes, an invalid symbolic interrupt trigger, repeated pauses in a loop reaching the same `PCLimitReached`, and zero-announcement guard precedence. Cumulative timeout and SMT-unknown tests must not yield a successful result. Every negative starts from an accepted baseline and requires the intended failure.

Run the affected native suite and the prior real SDK regression set against separately built new tools. Do not replace existing passing binary paths. Record commands, exits, logs, source/patch identities, and skipped cases. Inspect disk before builds. Coordinate expensive Cargo builds with the compiler worker and keep targets on internal storage.

## Completion

The prerequisite is complete only after the real public pause/resume path passes all listed cases and independent review accepts it. Linux EEI, software-thread scheduling, full roots, semantic coverage, and optimization equivalence remain separate requirements.
