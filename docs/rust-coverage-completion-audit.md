# Rust coverage completion audit

## Scope correction: 2026-09-08

The user defines Rust coverage as support for the compiled instructions through
Rust, object/ELF, Sail, Isla, and SMT for the selected target. This is separate
from a correctness proof for a particular program under its finite boundary.
The historical audit that follows combines these two questions. Do not use its
proof-obligation list as the instruction-support completion checklist.

The current target is `riscv64gc-unknown-linux-gnu`. The instruction-support
inventory must record the compiler flags, model configuration, and per-request
overrides. A base configuration alone does not establish the active extensions.
Each required instruction needs a source-backed path to its model operations
and native bindings. Record missing support separately from unmeasured support.

The current native regression log contains 73 distinct `No primop` names.
These are unresolved helper diagnostics, not 73 Rust features or 73 established
target gaps. Each name needs a caller, an extension, a fallback assessment, and
a selected-target classification. Helpers outside the selected target must not
hide required gaps. A test timeout is a separate acceptance result.

Current evidence:
`/Users/hak/hyperray-pipeline-campaign-20260908/full-rust-regression-20260908/full.jsonl`.
The package reached a terminal failure at 2026-09-08 02:50:11 UTC after
2237.902 seconds. Four FP tests failed. Three reported semantic resource limits.
The fourth still expects the old unavailable FP64-add binding. These results
do not establish complete instruction support. The 73-helper classification
remains in progress. No unchanged regression restart is necessary.

## Historical audit

Date: 2026-09-06. Status: full Rust coverage is not complete.

## Required result

For the closed finite boundary, every required compiled operation, root, machine case, external action, and proof obligation needs exact evidence. Passing program examples does not establish this result. `docs/design.md`, `docs/proof-machine.md`, and `docs/semantic-coverage.md` define the target. Old frozen specification files were deliberately removed in commit `c272c53`.

## Current evidence and missing connections

| Requirement | Current source evidence | Missing acceptance evidence |
|---|---|---|
| Compiler operation and root completeness | `adapters/rust/tools/mir-dump/src/rvalue.rs` maps `Repeat` and `ThreadLocalRef` to `Other`. `gates/leaf-rust.md` leaves roots and provenance open. | Compiler-generated complete inventory, exact valid entry predicates, and forward/reverse mappings for the selected Rust build. An opaque `Other` record is not a semantic mapping. |
| Executable instruction behavior | `machine/isla/executable_join.go` joins static footprints with observed execution. The real SDK tests exercise compiled Rust through the native tools. | Complete semantic-case equality and source/runtime provenance. Static instruction membership and observed events alone do not supply those proofs. |
| Runtime threads and external actions | The new `ProgramBoundary.Threads` declares initial machine threads. Current machine code has no Linux-user syscall, futex, or software-scheduler implementation. | General runtime/EEI connection for creation, join, scheduling, capacities, and external results within declared finite contracts. Explicit initial threads do not replace runtime thread creation. |
| Translation and coverage certificate | `coverage.Check` accepts an explicit `model.Model`. The executable result joins only Isla reports. `gates/leaf-circuit.md` records partial JIB/catalog and instruction-family evidence. | A general connection from compiler and Sail artifacts to the exact transition relation, plus an accepted certificate for that same relation. Do not generate a certificate from test counts or reuse a small explicit graph as the production machine. |
| Complete safety and liveness proof | `proof/` contains explicit graph reachability and cycle logic. `circuit/` contains relation/miter and solver proposal code. | Accepted production invariant/completeness evidence and liveness evidence for the machine model. The existing prototypes do not establish this connection. |

Unchecked gates can be stale. The audit therefore names implementation sources, not just checkbox counts. Some catalog infrastructure now exists even though older loader gates say it does not.

## Current coding task

The explicit initial-thread connection passed current acceptance and independent review at 08:20 UTC. The coordinator inspected logs for all three native tests, selected single-thread regressions, full Go results, focused negative tests, and binary hashes. `gates/leaf-isla-explicit-threads.md` records the evidence. Full Rust coverage remains incomplete.

The earlier guard binaries are absent from `/private/tmp/hyperray-execution-guards.JlGvKA`. A filesystem inspection at 08:01 UTC found no directory. Acceptance attempts before recovery reported `tool_not_found`. The remaining research binaries had different hashes and older capability versions.

Recovery used a new internal-disk build from the pinned upstream commit and the existing cumulative guard patch. Task `025373gj62` completed with exit 0 at 08:11 UTC. The dirty research checkout remains separate. The coordinator inspected the new binary identities and passing native results at 08:20 UTC.

The same-build compiler connection now has a written contract in `docs/plan-rust-same-build-inventory.md` and a separate Luna-high implementation worker. The pinned nightly lacked its RV64 library. Rustup task `0773112gw4` installed that matching component with exit 0. A later installed-target query lists `riscv64gc-unknown-linux-gnu`. This removes a toolchain prerequisite, not a coverage requirement.

## Reuse research: runtime connection

The official GenMC usage documentation provides an existing Rust compiler/runtime connection. It compiles Rust to LLVM IR and uses `genmc-std` for thread primitives. This is relevant source material for automatic thread actions.

However, the same documentation requires finite traces and data-deterministic programs. Nondeterminism must come from scheduling or memory order. That contract does not cover Hyperray's arbitrary declared finite input and external-choice sets. GenMC is not selected as a replacement proof authority.

Source inspected: https://mpi-sws.github.io/genmc/usage/ on 2026-09-06. The RustMC 2025 paper, https://arxiv.org/html/2502.06293, describes selective standard-library inlining and mixed-size-memory limitations. These are version-specific limits, not proof that future versions cannot improve them. No tool was installed or built for this research.

## Runtime source audit: 08:25 UTC

The existing production and research routes do not supply the required one-hart Linux-user runtime connection. `execution/native/search.pml:23` treats each whole native call as one search action. It does not expose internal scheduling points. `execution/nativecheck` provides search and replay diagnostics, not machine proof authority.

The existing GenMC checkout contains reusable compiler-to-action logic. `genmc-std/src/thread/genmc.rs:17-37` emits create/join primitives. `lli/Runtime/Execution.cpp:2989-3041` reads the LLVM values and calls the driver. `GenMCDriver.hpp:259-264` exposes thread-create and join actions. These rules are program-independent, but they bypass the selected Linux runtime. They do not supply object-to-action equivalence.

GenMC's local `doc/manual/usage.md:16-25` requires finite traces and data-determinism. `Execution.cpp:2895-2903` uses a per-thread PRNG value rather than all declared choices. Its replacement `thread/mod.rs:309-315` makes yield and sleep no-ops. The Loom research route also replaces imports. Neither route establishes this project's runtime contract.

Missing runtime primitives include a resumable one-hart execution interface, complete saved contexts, syscall effects, TLS, futex blocking and wakeup, a bounded software-thread table, finite external choices, and fairness/cycle evidence. `ExecutableVerifier.VerifyProgram` operates whole queries. Independent initial litmus threads do not provide those primitives. No inspected source supports a small runtime-glue assignment. The same-build compiler stage can proceed without a false runtime claim.

A later upstream executor audit found a defensible prerequisite: same-process pause/resume from frozen frames and paired solver checkpoints. `docs/plan-isla-continuations.md` defines its public contract and uninterrupted-versus-resumed acceptance. Luna high now implements a separate cumulative native patch. The pause occurs after instruction announcement, before instruction execution. This is not a Linux EEI or software scheduler.

## Next design decisions

1. Preserve the accepted initial-thread integration without declaring full Rust coverage.
2. Select a reusable runtime/EEI mechanism that matches the actual one-hart finite software-thread contract. Verify its behavior and limits from current source/documentation before assigning implementation. Do not substitute a multicore litmus test for a software scheduler.
3. Define the exact producer/consumer artifact connection for compiler roots, provenance, and machine coverage. Existing `coverage.Check` is an explicit-graph reference, not the missing production translation.
4. Connect the production transition model to independently accepted safety and liveness evidence. Keep failed or incomplete searches as errors without a root-level completion claim.

These are separate missing components. No source evidence supports describing the full objective as one small remaining SDK change.
