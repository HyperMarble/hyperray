# Trap boundary investigation

The invalid-stack regression returns an engine error at `trap_callback`.
That error prevents a proof result, but it does not give a modeled trap outcome.
The current integration must retain this safe error until a replacement preserves the fault behavior.

The pinned compiled model declares `trap_callback` as `unit -> unit`.
Its `trap_handler` calls that callback before the architectural trap changes.
The local upstream history supplies a matching empty callback in
commit `b6f7b1df64157e6b9d250e552842f963394bc2ba`, `model/riscv_callbacks.sail`.
This source is evidence for a notification callback, not permission to discard the trap.
The current upstream source has a different callback signature and cannot silently replace this snapshot.

The existing Isla configuration already supplies constant results for other notification callbacks.
A diagnostic configuration can supply the matching unit result for `trap_callback`.
The diagnostic must retain all later trap handling, register changes, and errors.
It must not skip the fault path, constrain the fault away, or turn an unknown result into a proof.

The diagnostic uses a copy of the recorded compiler-built function table.
Only its declared stack address changes to zero. The executable bytes remain unchanged.
The original model, configuration, program, and saved release artifacts remain unchanged.
This experiment does not change the production profile.

A successful replacement also requires explicit trap-state observations and a bounded handler contract.
An empty callback alone does not establish those requirements.

The first diagnostic leaves the handler registers unspecified, as the original configuration does.
The diagnostic log shows later decisions that depend on `mtvec`, `medeleg`, `PC`, and `nextPC`.
A second diagnostic declares `medeleg = 0` and sets `mtvec` to the recorded zero-instruction boundary.
This explicit diagnostic boundary ends at trap entry. It does not represent an operating-system handler or normal function return.
The required evidence is the model's fault-state changes before that boundary.
Neither diagnostic can establish the public trap contract by itself.

## Measured diagnostics

The unspecified-handler diagnostic exceeded its 120-second deadline and returned exit code 124.
It produced no completed trace or proof result.
The second diagnostic returned exit code zero and produced a 42,574-byte trace.
The trace retained these architectural writes before its stop boundary:

- `mcause = 0x0000000000000007`
- `mtval = 0xfffffffffffffff8`
- `mepc = 0x0000000080100020`
- `nextPC = 0x0000000080100048`.

The trace contains the initial stack adjustment and the next store instruction.
It does not continue through the normal function-table return path.
The files remain in `/tmp/hyperray-trap-boundary.BBILdt/`.
`configuration.toml` adds only the notification callback to the original configuration.
`program.toml` changes only the initial stack address in the recorded program.

This result is a trace, not a proof of a fault requirement.
At this diagnostic stage, the public sequential assertion validator accepted bitvector final registers, not fields in the structured `mcause` register.
The public result also needs a distinction between normal completion and the declared trap boundary.
These contracts remain open. The production invalid-stack regression remains unchanged and returns an explicit error.

## Public typed-state connection

The public program boundary now carries typed initial registers through the existing Isla value parser.
The fault-address test uses that input connection for `mtvec` and `medeleg`.
Both public proof queries passed, including a concrete fault-address counterexample.
`docs/isla-initial-state.md` records this result.
Structured final-register assertions and trap-completion classification remain separate, unfinished requirements.

The existing `--trace-function trap_handler` option also retained an explicit call and return around the architectural fault writes.
`/tmp/hyperray-initial-state.BtoOF9/trap-events.trace` records that successful diagnostic.
These model events can distinguish trap handling from a program that merely writes a cause register.
The integration must retain path constraints when it turns such an observation into a counterexample.
An observed call alone does not establish that its path is feasible.

## Solver-backed call observations

The forbidden-call connection now retains these events in the candidate query.
The solver accepts a fault witness only under the original path constraints.
`TestRealRustForbiddenTrapCall` returned `called:trap_handler=true;` through the public SDK.
The normal-return test returned `called:trap_handler=false;` for a changed requirement.
Both tests asserted the same values in the public `ModelCalls` map.
The native solver test rejected a contradictory path that contained a forbidden call.

All twenty real-tool tests passed with this connection.
`docs/isla-forbidden-calls.md` records the scope and measurements.
Structured final-register assertions and complete exit classification remain open.

## Structured cause connection

The register-field change completes the scalar cause-assertion connection.
The public SDK proves `mcause.bits = 7` for the declared fault boundary.
It rejects a changed cause requirement and returns the concrete cause.
The combined safety query retains the cause and the feasible `trap_handler` call.
`docs/isla-register-fields.md` records the source basis and test results.
The full exit classification and handler-contract rejection matrix remain open.
