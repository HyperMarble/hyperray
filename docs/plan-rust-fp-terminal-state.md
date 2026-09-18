# Rust FP acceptance repair: terminal state

## Scope

Connect the existing Rust ELF, Sail/Isla execution, and SMT assertions. Do not replace floating-point arithmetic based on unrelated diagnostics. Do not claim full Rust coverage from this repair.

## Current evidence

Source root: `/Users/hak/hyperray-isla-fp-task-a-20260907`.

- `isla-axiomatic/src/axiomatic.rs:971` populates `final_writes` from `WriteReg` events only.
- `isla-axiomatic/src/register_fields.rs:52` rejects a register absent from that map.
- `isla-lib/src/executor.rs:2020` discards the terminal frame in `trace_collector`.
- `isla-lib/src/executor/frame.rs:201` exposes `regs()`.
- `isla-lib/src/register.rs:303` exposes the non-mutating `get_last_if_initialized` accessor.
- `isla-axiomatic/src/run_litmus.rs:371-377` prunes SMT definitions and renumbers symbols. Any terminal values must remain paired with these operations.

The measured log is `/Users/hak/hyperray-pipeline-campaign-20260907/floating-positive-final.jsonl`. Exact and sticky cases fail with `Final register has no recorded value`. The inexact case passes. The `parse_hex_bits` message is a load warning, not evidence that this primitive caused these failures.

## Required semantics

A final assertion observes actual terminal state. An initialized register does not disappear because the instruction leaves it unchanged. The proof and its counterexample must use the same value source.

Capture whole terminal register values without architectural writes or reads added for evidence. Preserve initialized symbolic values and their SMT definitions. Retain thread and path ownership. Keep absent or unsupported state as an explicit error. Do not infer whole state from partial-accessor events. Do not treat relaxed registers as ordinary registers without examining their documented semantics.

Use paired terminal metadata, not additional architectural events. One path record owns its trace and required initialized whole-register values from the terminal frame. Preserve terminal symbols as explicit roots during SMT pruning. Apply the same symbol renumbering to values and trace. Candidate selection must carry the matching state record by construction.

The terminal snapshot is authoritative, not a fallback for missing writes. Symbolic-index register writes can change frame state through `Abstract` events without a `WriteReg` event (`isla-lib/src/executor.rs:329-346`). Thus an event-only final map can be stale as well as absent. Use the snapshot for both assertions and counterexamples. Keep existing instruction, register-event, and memory-model relations unchanged. Independent review must examine relaxed-register behavior and error propagation.

## Acceptance

1. Initialized, unwritten struct fields remain observable.
2. A later write overrides the initial value.
3. Multiple and partial writes preserve the correct whole state and siblings.
4. Symbolic terminal fields retain declarations and constraints after renumbering.
5. Missing state produces an exact error.
6. Separate paths and threads do not share terminal values.
7. Proof and counterexample use identical terminal values.
8. Real Rust exact-add, sticky-flag, and inexact-add tests pass with the same measured ELF and rebuilt native tools.
9. Wrong-result and wrong-state assertions produce the expected counterexamples.
10. Independent review and existing native regressions pass before a small commit.

## Test evidence rules

Two reviewed tests declared and defined the same SMT symbol. A string or event-count assertion did not expose this invalid baseline. Use distinct symbols for inputs and derived values. Replay the exact retained and renumbered SMT before a test can support an acceptance claim. Require a satisfiable input baseline, an unsatisfiable wrong-result query, and an actual decoded witness where the contract requires one.

Generated query text does not prove a solver result. Scalar registers do not test sibling fields of a struct. Recreating an iterator does not test its next candidate. Each test must execute the operation named by its requirement.

## Separate failures

The old unsupported-add test omits `fcsr`. The selected IR reads its FRM field for dynamic rounding and rejects modes 5, 6, and 7. Its illegal-trap path needs separate diagnosis. Do not suppress `trap_callback` to turn this into an add success.

`machine/isla/real_input_test.go:37` sets a 10-second query limit. `verifyRustBoundary` also supplies a 60-second verifier limit, but that does not replace the query limit. Symbolic acceptance failures need time/output measurements before any limit change. Preserve the same input and PC bounds.

The previous full real suite ended at its 600-second wrapper timeout. It is incomplete. Resume only unrun or unterminated cases after making sure that the old process is absent. Preserve all existing failures.
