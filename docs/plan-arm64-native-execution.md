# ARM64 native execution contracts

Status: proposed design, 2026-09-08. No ARM execution acceptance is claimed.

This document supplements [the macOS connection plan](plan-arm64-macos-connection.md).
It defines four logical patches. Each patch needs its own acceptance evidence
and independent review before commit. Proposed fields and APIs do not exist yet.

## Scope and source identity

The target remains real compiler-produced ARM64 Mach-O bytes through Isla and
SMT. An internal ELF carrier can transport those exact bytes. A Linux rebuild
cannot replace the Mach-O artifact.

The inspected sources are:

- Native source root: `/Users/hak/hyperray-isla-fp-task-a-20260907`
- ARM IR: `/Volumes/Hak_SSD/hyperray-research/isla-snapshots/armv8p5.ir`
- Configuration: `/Volumes/Hak_SSD/hyperray-research/isla-7f6882b/configs/armv8p5.toml`
- Go source root: `/Volumes/Hak_SSD/hyperray`.

Native paths in this document are relative to the native source root.
IR line numbers refer to the specified ARM IR. They are inspection references,
not stable revision identifiers. The isolated native tree has no `.git` metadata.
The last snapshot commit for the IR is `d1cee06`, `Add new IR files`.

Each acceptance record must bind the Mach-O digest, selected slice, query,
ARM profile, IR digest, configuration digest, CAT input, and native binary hashes.
Configuration identity includes caller state after reset. A filename or version
string does not establish compatibility. New behavior needs a new capability
identifier and measured binaries. Existing preserved binaries do not acquire
these capabilities through a configuration change.

## Existing state boundary

ARM names are `_PC`, `R0` through `R30`, and `SP_EL0` for this EL0 profile.
The configuration maps `X0` through `X30` to `R0` through `R30`. The link register
is `R30`. The footprint invocation must use `_PC`, not the current Go `PC`.

IR `zmain` at 629840 calls `zinit`, `sail_reset_registers`, and then the instruction
loop. `zinit`, near 629784, calls `TakeReset(COLD_RESET)` and assigns these values:

- `_PC = elf_entry()`
- `SP_EL0 = 0x3c00`
- `PSTATE.EL = EL0`
- `PSTATE.PAN = 0`
- `DBGEN = LOW`.

Reset also changes control, SIMD, FP, and debug state. Native `run_litmus.rs`
inserts `initial_state` and thread `init` before `main`. Those values do not
establish caller state after reset.

Existing thread `reset` data applies through `TaskState.with_reset_registers`.
`isla-lib/src/executor.rs:686-724` applies it after model initialization.
A task reset overrides the same location in the configuration reset.
The Go renderer does not expose this distinction yet. The connection stage
must expose caller state after reset rather than silently accept an overwritten SP.

The configuration resets `SCTLR_EL1` to `0x0000000004000000`.
This is a model assumption, not a Darwin process state.

## Patch R1: native continuation boundary

### Existing interception point

`isla-lib/src/executor.rs`, `run_loop`, handles `Instr::Call` near line 1285.
The defined-function branch has the frame, solver, task state, task ID, worker
queue, timeout, and task fraction.

ARM `__fetchA64` is a real `unit -> bv32` function at IR 59477.
`Step_CPU` calls it at 628792. Its body does software-step and alignment checks,
then address translation and memory access, then an illegal-state check.

The proposed interception occurs before entry into this resolved function.
It precedes call-stack changes, call-event publication, function assumptions,
abstract calls, and the function body. It does not replace instruction semantics.
It defines the boundary before the next fetch, not successful execution of that fetch.

A memory-error handler is not a return boundary. A `read_mem_ifetch` interception
uses a physical address and occurs after translation. Instruction announcement
occurs too late. ARM exits on a zero instruction at IR 628855-628865, before
`sail_instr_announce` at 628869. Disabling generic `zero_announce_exit` does not
remove this earlier model exit.

### Proposed reachable input

An optional thread `return_address` TOML string supplies the continuation.
Native parsing carries it through thread initialization into `TaskState`.
The native TOML/CLI path is the public caller for this patch.

The supported ARM profile resolves `_PC` and `__fetchA64` from the loaded symbol
table. Setup validates the register width and function signature. A caller cannot
select an arbitrary model function as a successful return hook.

Setup rejects conflicting hooks, unsupported profiles, misalignment, and an entry
address equal to the continuation. The first profile reserves the continuation
outside loaded executable ranges. No byte at that address is necessary.

### PC equality and symbolic paths

The hook observes the current `_PC` without a register assignment or synthetic
architectural read/write event. Existing `get_last_if_initialized` supports this
observation. Missing, malformed, or wrong-width PC state produces an error.
The hook must not create a missing PC value.

Equality uses the full 64-bit architectural PC, not a truncated or physical address.
For symbolic or mixed values, the existing bitvector-to-SMT conversion supplies
an expression. Let `E` mean `PC == return_address` under the current path constraints.

| Feasible `E` | Feasible `not E` | Required outcome |
|---|---|---|
| Yes | No | Retain `E` and return a distinct boundary outcome. |
| No | Yes | Retain `not E` and execute the original fetch. |
| No | No | Return `Run::Dead` for this inconsistent path. |
| Yes | Yes | Preserve both paths with the existing task-fork machinery. |

The two-path case uses the pattern in `Instr::Jump`, executor lines 1168-1214.
The equality predicate needs a Boolean SMT symbol for `Event::Fork`.
The queued non-return task retains the same call PC and a `not E` fork condition.
The current task retains `E` and returns the boundary outcome.
When the queued task resumes, equality is infeasible and the real fetch proceeds.

The fork must preserve the checkpoint, task fraction, task state, stop conditions,
path identity, and event history. It must not assign a concrete value to `_PC`.
A solver model sample alone is not evidence of equality. Neither branch can vanish
because its execution is inconvenient. An unsupported non-return target remains
an execution error, not a discarded path.

### Outcomes and error precedence

A proposed `Run` outcome such as `BoundaryReached(address)` distinguishes the
boundary from `Run::Exit` and `Run::Finished`. The exact API name is not prescribed.

The existing loop timeout check precedes dispatch. Earlier instruction errors
retain precedence. Solver `Unknown`, malformed PC, and unsupported state remain
errors. Only proven-inconsistent paths become `Dead`.

On a non-return path, existing fetch, alignment, translation, memory, timeout,
and PC-limit behavior remains unchanged. In a boundary-required query,
`Run::Exit` and `Run::Finished` produce distinct unexpected-termination errors.
`PCLimitMode::Discard` is not permitted for this proof profile. An empty set of
successful paths cannot establish a proof.

This contract establishes arrival at a declared continuation. It does not establish
that the preceding instruction was `RET`, nor call-stack integrity. Branch provenance
is a separate requirement if the product needs that stronger claim.

The finite no-exception profile must retain exception observations. Existing
`forbidden_model_calls` requires resolved, instrumented functions. Calls such as
`AArch64_TakeException` or `AArch64_Abort` must falsify the safety requirement or
produce an error. A later arrival at the continuation cannot erase that evidence.

### Required acceptance

- An equal, unmapped continuation produces no fetch event and preserves result and SP.
- An unequal, unmapped target retains the original execution error.
- A symbolic PC constrained equal returns without a PC assignment.
- A symbolic PC with both feasible outcomes retains both paths.
- An error on the non-return path prevents a successful whole-query result.
- Solver `Unknown` remains an error.
- Zero instructions, model exit, natural model completion, timeout, and PC exhaustion do not count as return.
- Nested machine calls continue until the declared continuation.
- Queries without a return boundary retain existing RISC-V behavior.

## Patch R2: terminal evidence through native output and Go

### Existing transport

`isla-lib/src/executor.rs:2009` defines `TraceRecord` with events and terminal registers.
At 2039, `trace_collector` merges `Run::Finished` and `Run::Exit` into that record.
It currently loses the termination reason.

`isla-axiomatic/src/run_litmus.rs:370-384` retains terminal values as SMT roots
and renumbers them with their events. `CandidateIter` and `ExecutionInfo::from`
in `isla-axiomatic/src/axiomatic.rs` consume paired path records.
Native `src/axiomatic.rs`, `print_results_herd7` near 1038-1066, emits candidate status.

### Proposed contract

R1 must retain a terminal kind beside the authoritative terminal snapshot.
R2 carries that metadata through candidate output and Go. Each path owns its reason,
thread identity, and declared continuation. A global flag such as “one path returned”
is insufficient.

The terminal metadata can contain only a concrete declared address and reason.
The actual `_PC` remains in `terminal_registers`, with its existing SMT roots and
renumbering. If metadata contains symbolic values, the same transformations must
also apply to those values.

A new versioned output record must associate terminal evidence with its candidate
and thread. Both SAT and UNSAT candidate records need terminal evidence.
A SAT witness obtains actual PC from the existing solver-model decoder. It must
not substitute the declared continuation constant as the observed value.

Go `parse_candidates.go` and `parse_candidate_state.go` need strict parsing for
these records. Missing, duplicate, malformed, unknown-version, or mismatched records
produce errors. `ExecutableResult` and `VerificationResult` must expose the terminal
evidence through public fields or methods. `ProgramEvidence` must bind the expected
boundary and ARM profile.

The existing `BuildARM64Program` proposal must return the existing opaque `Program`.
A caller then uses the existing `VerifyProgram` path. Profile identity prevents an
ARM program from pairing with a RISC-V release. Capability checks and binary hashes
must require the new transport behavior.

Proof acceptance requires the expected boundary for every feasible accepted path.
It must not infer universal return from one witness. Native errors remain errors
with no verdict. Proof and counterexample use the same terminal-state source.

### Required acceptance

- Correct-result UNSAT and wrong-result SAT cases carry equivalent terminal semantics.
- The SAT witness PC comes from the retained solver state.
- Separate threads and paths retain their own reasons and addresses.
- SMT pruning and renumbering preserve symbolic terminal PC values.
- Missing, duplicate, mismatched, or unsupported terminal records produce errors.
- A caller outside the Go package can construct inputs and observe terminal evidence.
- Existing RISC-V constructors and results remain compatible when no boundary is present.

## Patch M1: ARM request-kind validation

### Exact source findings

IR 773-815 defines these request components:

- `Access_variety`: `AV_plain`, `AV_exclusive`, `AV_atomic_rmw`
- `Access_strength`: `AS_normal`, `AS_rel_or_acq`, `AS_acq_rcpc`
- `Access_kind`: `AK_explicit`, `AK_ifetch`, `AK_ttw`, `AK_arch`
- `Mem_read_request` and `Mem_write_request`, with address, size, tag, and translation fields.

`AccType_to_Access_kind`, near IR 2892, maps ordinary operations to explicit/plain/normal.
It maps `ATOMICRW` to atomic RMW and `PTW` to table walks. Vector, stream,
unprivileged, and cache-related operations use architectural variants.
`ORDEREDRW` obtains an undefined kind in this snapshot.

The exclusive classifiers at IR 2756-2818 recognize only `AV_exclusive`.
Thus atomic RMW can enter ordinary `read_mem` or `write_mem` with
`opts.is_exclusive == false`. Existing sequential callbacks ignore the request kind.
The current exclusive flag alone cannot establish a permitted ordinary access.

### Proposed first kind policy

| Request | Admission |
|---|---|
| Read `AK_ifetch` | Matching fetch flags, no tags, and immutable initialized instruction bytes. |
| Data read/write `AK_explicit(AV_plain, AS_normal)` | Matching ordinary flags and valid request data. |
| `AV_exclusive` or `AV_atomic_rmw` | Explicit unsupported-access error. |
| `AS_rel_or_acq` or `AS_acq_rcpc` | Explicit unsupported-access error. |
| `AK_ttw` | Explicit unsupported-access error. |
| All `AK_arch` variants | Explicit unsupported-access error in this first profile. |
| Malformed, unsupported symbolic, or undefined kinds | Explicit error. |
| Request kind inconsistent with primitive flags | Explicit error. |

This table defines a narrow first profile. It does not complete ARM coverage.
Vector, ordered, atomic, and unprivileged operations remain requirements for later
independent semantic patches. The model retains instruction behavior for accepted kinds.

### Placement before fast paths

`Memory.read`, `isla-lib/src/memory.rs:454`, calls `validate_read` before
`read_initialized` and concrete/custom paths. `Memory.write`, near 583, calls
`validate_write` before permission checks and custom writes.

The proposed change passes the actual request kind and necessary schema context
to those validators. Validation only in `symbolic_read` or `symbolic_write` is
insufficient. `initialized_memory/loaded_read.rs` can return concrete bytes first.

Setup resolves field and constructor IDs from the loaded symbol table and type
information. It must not use guessed numeric IDs or debug-string comparisons.
Explicit profile dispatch retains current RISC-V behavior.

Malformed address, size, value, or tag relationships produce errors before event
publication or byte-array mutation. Existing permission and overlap errors remain
unchanged for accepted requests. Unsupported kind errors precede memory fast paths.

### Store status requires no new constraint

`sail_mem_write`, IR 3162-3271, returns `Ok(Some(primitive_boolean))`.
Its sole caller, `__WriteMemory`, IR 12311-12337, matches `Ok(_)` and discards the
payload. Only `Err(FaultRecord)` calls `AArch64_Abort`.

Ordinary ARM stores therefore need no constraint that forces the primitive Boolean
true. They need no status-conditional array update. This finding does not authorize
exclusive or atomic-RMW semantics.

### Required acceptance

- Actual ARM ordinary stores feed subsequent reads, including overlapping widths.
- Concrete and symbolic addresses preserve byte order and forwarding.
- Unsupported kinds fail before initialized, concrete, custom, or symbolic memory paths.
- Atomic RMW fails even when the primitive exclusive flag is false.
- Request/flag mismatches, tags, and malformed values produce errors.
- Rejected writes preserve memory and publish no successful write event.
- Read-only overlap and unknown solver results retain existing errors.
- Ordinary store behavior does not depend on the unused primitive Boolean.
- Existing RISC-V memory acceptance remains unchanged.

## Patch M2: memory attributes and explicit ARM admission

### Normal-versus-Device caveat

Absent translation metadata does not establish Normal memory.
The stage-1-disabled path initializes `translation_info = None` at IR 44028,
then calls `AArch64_TranslateAddressS1Off` at 44039.

In that function, the Normal-data branch requires
`HasS2Translation() && HCR_EL2[12] == 1`.
Otherwise non-fetch accesses receive `MemType_Device` and `DeviceType_nGnRnE`
near IR 32782. Instruction fetch receives Normal attributes near 32625.

The pinned configuration has `HCR_EL2 = 0` and `SCTLR_EL1.M = 0`.
It is not evidence of a Normal-memory Darwin process profile.
The exact post-reset control state still needs execution evidence.

When translation metadata is `None`, the memory request omits
`AddressDescriptor.memattrs`. M1 cannot enforce a Normal-only policy from that
request alone. Accepting `None` as Normal silently loses a material condition.

### Proposed descriptor validation

Existing `aget__Mem` at IR 58786 and `aset__Mem` at 58320 receive an
`AddressDescriptor`, optional translation information, size, and `AccessDescriptor`.
The write function also receives the value.

An ARM-specific validator at these resolved call boundaries can inspect the descriptor
before the metadata disappears. It rejects Device, tagged, malformed, and unsupported
symbolic attributes for a Normal-only profile. Accepted calls retain the original model
bodies and their errors. The validator must not replace returned values or suppress faults.

The memory-policy patch must account for routes that bypass these functions.
A low-level request with missing attribute provenance cannot inherit approval from
an unrelated earlier access. M1 remains mandatory at every primitive memory path.

A source-backed caller state after reset must establish the intended Normal-memory
profile without unmodeled page-table effects. Setting an HCR bit alone is not yet a
validated solution. Security, regime, and `HasS2Translation` conditions also matter.
This control-state profile remains an open blocker.

A declared RAM device with Device attributes is an alternative contract. It needs
separate evidence and cannot become an implicit fallback for Normal RAM.

### Admission after evidence

`isla-axiomatic/src/sequential_setup.rs:16` currently requires `arch == "RISCV"`
and one thread. ARM admission must require the exact supported ARM profile and
M1/M2 behavior. It must not merely remove the architecture test.

Existing exclusions remain: multiple threads, interrupts, page-table effects,
translation rewrites, self-modification, and unsupported memory layouts.
The model/configuration/profile identity must make the attribute assumptions explicit.

### Required acceptance

- A Device descriptor produces an explicit rejection under the Normal-only profile.
- `translation_info = None` cannot bypass attribute validation.
- Unsupported symbolic or tagged attributes cannot silently become ordinary RAM.
- The selected post-reset profile produces measured Normal attributes for real accesses.
- Accepted accesses retain model alignment, permission, and exception behavior.
- Unsupported low-level paths cannot reuse attribute approval from another access.
- ARM admission requires the new profile and capabilities.
- Model, configuration, or profile mismatches produce errors before a verdict.

## Separate mapping and runtime requirements

The sequential byte array leaves unspecified data bytes symbolic. Current loaded
memory checks protect initialized read-only bytes. They do not establish an OS
address space, bounded stack, allocation lifetime, or guard pages.

A bounded-memory safety claim needs explicit region and permission enforcement,
including symbolic range reasoning. Store forwarding alone does not meet that claim.

These four patches do not supply dyld, libSystem, Darwin syscalls, TLS, allocation,
signals, threads, or framework dependencies. Local machine calls need their actual
bytes and state. Imported runtime operations remain explicit blockers until their
real contracts and implementations exist. A closed function proof does not complete
ordinary macOS process support.

## Evidence status

This design derives from local source inspection on 2026-09-08.
No native builds, solver tests, program execution, or downloads supported these claims.
The acceptance cases in this document are requirements, not passed tests.
