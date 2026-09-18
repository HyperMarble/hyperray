# Sequential terminal-memory transport

## Goal

Carry the final symbolic byte array from native sequential execution into each
trace path, candidate, and `ExecutionInfo`. Keep this work separate from
instruction semantics and from the setup `Memory` object.

## Source facts

- `SequentialMemory.memory` stores the current array symbol.
- Sequential writes replace that symbol with a `Store` chain.
- Sequential reads constrain values with little-endian `Select` and `Concat`.
- `LocalFrame::memory()` exposes the read-only runtime memory object.
- `TraceRecord` already carries initialized terminal register values.
- `run_litmus_setup` prunes event roots and renumbers terminal registers.
- `ExecutionInfo::from` transfers terminal registers into `final_writes`.
- `final_assertion::equality` already emits `last_write_to_N addr value`.
- `final_state_from_z3_output` is a model reader for axiomatic output. It uses
  the wrong one-argument `last_write_to_N` interface and must not be reused.

## Design

1. Add a default read-only snapshot callback to `MemoryCallbacks`.
   `SequentialMemory` returns its current array symbol. Other memory clients
   return no snapshot.
2. Add a read-only `Memory::terminal_snapshot` accessor. It returns a
   `Val::Symbolic` array root when a callback supplies one.
3. Add `terminal_memory: Option<Val<B>>` to `TraceRecord`. Capture it in the
   `BoundaryReached` and `Finished` or `Exit` branches of `trace_collector`.
   Footprint records carry `None` because they do not use sequential memory.
4. Add terminal memory to the roots passed to
   `simplify::remove_unused_with_roots`. Apply the same per-thread symbol
   renumbering as terminal registers. This keeps each path's events and array
   snapshot paired after pruning and fork renumbering.
5. Carry the path snapshot through `Candidate` and into `ExecutionInfo` as a
   per-thread borrowed value. Do not replace `Candidate.memory`, which remains
   the setup memory used by page-table and initial-memory analysis.
6. Extend sequential assertion validation to accept `LastWriteTo` only with a
   terminal memory snapshot, a concrete literal address, a width of 1, 2, 4,
   or 8 bytes, and a complete final array observation. Reject missing snapshots,
   unsupported widths, and invalid addresses as values before SMT output.
7. Emit named `last_write_to_8`, `16`, `32`, and `64` Boolean predicates over
   the final array observation. Each predicate reads bytes in little-endian
   order and compares the requested value at the requested address. Keep the
   existing two-BitVec64-argument predicate interface used by
   `final_assertion::translate`.
8. Use the final array root for these observations. Do not infer memory state
   from the latest write event. Do not add the old one-argument model lookup.
9. Preserve the no-snapshot behavior: sequential final-memory assertions fail
   with an explicit error when the callback did not provide an array root.
   Register assertions continue to use initialized terminal values, including
   initialized-but-unwritten registers.
10. Add a reachable native binding for observation names. Parse repeated
    `[[memory_observations]]` tables with exactly `name`, `address`, and
    `bytes`. Bind each name to its checked literal address and width before
    final assertion evaluation. The existing `*<name>` expression grammar
    remains unchanged. Unresolved names return an explicit error and never
    become address zero. Observation metadata does not initialize memory or
    add execution effects.
11. Keep ARM identity VA=PA only inside the existing normal-memory page-table
    path. Do not apply that identity as a general address fallback.
12. Emit named `last_write_to_8`, `last_write_to_16`, `last_write_to_32`, and
    `last_write_to_64` Boolean predicates over the final array observation.
    Each predicate reads bytes in little-endian order and compares the
    requested value at the requested address. Keep the existing two-BitVec64-
    argument predicate interface used by `final_assertion::translate`.
13. Use the final array root for these observations. Do not infer memory state
    from the latest write event. Do not add the old one-argument model lookup.
14. Preserve no-snapshot behavior. A sequential final-memory assertion fails
    with an explicit error when the callback provides no array root. Register
    assertions continue to use initialized terminal values, including
    initialized-but-unwritten registers.

## Acceptance checks

- A real litmus input parses a named observation and a `*name` final assertion.
- A final store with no later load keeps its array definitions and satisfies
  the matching assertion.
- A correct assertion's negation is UNSAT. A wrong assertion's negation is SAT
  when the wrong value is present, using named scalar get-value output.
- The scalar output decoder returns the concrete final observed value.
- A partial overlapping store preserves unaffected bytes and overwrites only
  the stored bytes in little-endian order.
- Initialized but unwritten register assertions still use the terminal state.
- Forked paths keep their own snapshots and event symbols after pruning and
  renumbering. Two candidates retain distinct path/thread pairing.
- Missing snapshots, duplicate or unresolved names, bad widths, and bad
  address extents return explicit errors.
- The collector-to-candidate-to-`ExecutionInfo` path is tested without running
  native builds in this worker. Root grants the native build and test slot.

## Files

- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-lib/src/memory.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-lib/src/sequential_memory/callbacks.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-lib/src/sequential_memory/mod.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-lib/src/executor.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/litmus.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/litmus/exp.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/run_litmus.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/axiomatic.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/sequential_candidate.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/sequential_candidate_assertion.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-axiomatic/src/smt_events.rs`
- `/Users/hak/hyperray-isla-fp-task-a-20260907/isla-lib/src/simplify.rs`
- Existing terminal transport tests will be extended in
  `isla-axiomatic/src/terminal_transport_tests.rs`.

No Go API, fixture, Sail opcode, or native execution changes belong here.
