# Stage 3 — MODEL

Status: `REPLACED` and `NOT COMPLETE` 2026-09-04.

This design replaces the Rust Kani/GOTO loop-inventory design. Stage 3 builds
the language-neutral reference model in `docs/proof-machine.md`.

The former implementation work remains useful as measured accelerator work.
It does not satisfy the semantic coverage contract.

## 1. Purpose

Stage 3 converts one source build and one closed finite boundary into a finite
machine and environment transition system.

It also proves that the transition system has complete semantic coverage. It
does not calculate reachability or issue a logic verdict.

Stage 4 receives the model only after the coverage checker accepts its
certificate. Every earlier problem returns an engine error.

## 2. Input

Stage 3 receives these artifacts:

- The base source and applied solution patch
- The named requirements and their finite-state monitors
- The closed finite boundary, root catalog, and query-specific root sets
- The pinned compiler, linker, runtime, libraries, target, and build flags
- The Stage 1 source manifest and Stage 2 lint findings.

The Stage 1 manifest selects report locations. It is not the compiler operation
inventory for semantic coverage.

Stage 2 findings remain style data. They cannot add or remove proof behavior.

## 3. Compiler build and inventory

Stage 3 compiles the entire program and runtime for the selected build identity.
It keeps the final executable image.

The compiler inventory contains every compiled operation instance. It includes
concrete function instances and compiler-generated runtime operations.

Before reachability, each operation records `mapped` or
`eliminated_with_proof`.

Stage 4 can refine a mapped operation to `unreachable_with_proof` for one
query. This result keeps the machine mapping and cannot reduce semantic
coverage.

The linker inventory also contains startup code, thunks, and other synthesized
executable operations. Each synthetic operation has an explicit kind.

Every operation has a stable identifier and provenance. Every artifact records
its digest and producer version.

A compiler-eliminated operation requires a checked elimination record. Stage 3
cannot treat silence as optimization evidence.

## 4. Provenance and transition mapping

Every `mapped` compiler operation has this mapping:

```text
compiler operation
  -> compiler output record
  -> executable machine instruction or instructions
  -> machine transition or transitions
```

An `eliminated_with_proof` operation maps to its compiler elimination record
and the equivalent surviving transitions.

The reverse mapping also holds. Every executable instruction maps to an
inventoried operation or a declared synthetic operation.

Every model transition maps to a machine instruction or a declared environment
operation. No transition has an unknown source.

An operation with `unreachable_with_proof` remains in these maps. Stage 3 does
not use reachability to change the coverage denominator.

## 5. Machine and environment model

The model contains machine, memory, runtime, scheduler, environment, and
requirement-monitor state. All state domains are finite.

Memory state covers allocation, free, address reuse, pointer provenance,
uninitialized data, and garbage collection.

Concurrency state covers recursion, stacks, thread and task creation, joining,
cancellation, signals, data races, atomic operations, fairness, and progress.

Machine transitions use the selected target instruction semantics. Concurrent
transitions use the selected memory model.

Scheduler transitions preserve every allowed choice. Environment transitions
preserve every declared result, event, exception, and callback.

An unknown instruction, runtime operation, system call, foreign call, or
environment action returns an engine error. A silent stub is not permitted.

Normal termination, deadlock, trap, abort, and capacity errors remain different
states. The model does not merge their effects.

## 6. Function entry states

Each in-scope function instance has exact standalone entry information. This
rule also applies to instances outside program-start reachability.

A function normally has a nonempty valid entry set. An impossible precondition
requires a checked proof, not a fabricated or silently empty root.

An entry state includes these values:

- The arguments and receiver
- The global, heap, alias, and object state
- The register, stack, thread, scheduler, and runtime state
- The environment and requirement-monitor state.

An entry generator must prove that it represents the exact declared finite
domain. A missing root or partial generator returns an engine error.

Each named requirement selects only its applicable roots. An unrelated function
root cannot enter that proof query.

## 7. Semantic coverage certificate

The coverage checker recalculates each required equality. It rejects missing,
duplicate, stale, or unknown references.

The certificate proves these facts:

- Every inventory operation has one valid base disposition.
- Every `mapped` operation has output, instruction, provenance, and transition
  mappings.
- Every `eliminated_with_proof` operation has a checked elimination proof and
  equivalent surviving transitions.
- Every `unreachable_with_proof` query annotation retains its `mapped`
  operation evidence.
- Every executable instruction has provenance and transition semantics.
- Every model transition has a mapped machine or environment operation.
- Every in-scope function instance has exact entry information.
- Every proof query selects only its applicable roots.
- Every state component and nondeterministic choice has a finite domain.
- Every unreachable operation remains in the coverage map.

Only a complete coverage certificate can enter Stage 4. A loop list, harness
list, line metric, test result, or mutation score cannot replace it.

## 8. Language routes

Rust, C, C++, Go, and Python use the same reference machine. Each route compiles
its source and runtime into one executable model.

The first profile uses a static, little-endian RV64 Linux ELF. It uses LP64D,
RVA20U64, one physical hart, and a finite Linux-user environment.

The RISC-V manuals define instruction behavior. A pinned Sail model supplies
the executable reference. Each BTOR2 rule requires a one-step equivalence
result.

The Python route includes the pinned interpreter, runtime modules, and native
libraries. Each inventoried code object has exact entry information.

Dynamic loading or code generation requires a finite model of the generator
and every permitted result. Otherwise, Stage 3 returns an engine error.

Each source-language query includes `NO_UNDEFINED_BEHAVIOR`. Hyperray must prove
this requirement before another source-level `PROVED` result.

## 9. Accelerator data

The Rust adapter does not infer numeric loop limits from MIR patterns.
The adapter calls `rustc` through the existing compiler driver.
Structural MIR cycle reports remain diagnostic data, not bound evidence.
Kani generates the accelerator model. CBMC supplies its loop inventory.
Neither a cycle report nor a loop inventory supplies a proven numeric bound.
The removed linear-loop proposal fields are not part of the adapter output.

Kani and CBMC are accelerators for the Rust route. They are not a coverage
authority and cannot issue a reference verdict.

Kani can run with `--only-codegen` and produce a GOTO model for a selected
harness. CBMC can list loops in that model.

Stage 3 can retain these accelerator facts:

- The Kani arguments and version
- The harness classes, attributes, contracts, selections, and skip reasons
- The final linked `.out` path
- The CBMC version and GOTO loop list
- The accelerator artifact digests and source provenance.

A Kani skip does not remove a function root. A GOTO loop list does not cover
compiler operations, executable instructions, environment transitions, or
entry states.

An accelerator counterexample must replay in the reference transitions. A
positive accelerator proof requires checked equivalence and an independently
accepted proof certificate.

Unwinding assertions describe the selected accelerator harness. They do not
establish complete semantic coverage for the source build.

## 10. Output

Stage 3 produces versioned boundary, model, and coverage artifacts. Their exact
schemas live under `schemas/`.

Together, the artifacts identify these items:

- The build identity and executable image
- The compiler and linker inventories
- The provenance graph
- The finite state and transition definitions
- The root catalog and query-specific root sets
- The requirement monitors
- The observation alphabet and observation functions
- The semantic coverage certificate
- The stated trust assumptions
- The retained accelerator telemetry.

An engine error identifies its failed stage, cause, artifact references, and
tool versions. It contains no logic verdict.

## 11. Completion gate

Stage 3 remains incomplete until all these conditions have measured evidence:

- Every inventory operation has a valid base disposition.
- Every mapped operation and executable instruction has total provenance and
  transition mappings.
- Every eliminated operation has a checked elimination proof.
- Every reference transition has executable or declared synthetic provenance.
- Every in-scope function instance has exact entry information.
- Every proof query selects only its applicable roots.
- Every machine, runtime, scheduler, and environment operation has exact finite
  semantics.
- The checker accepts the complete semantic coverage certificate.
- Unreachable operations remain mapped and receive an explicit reachability
  value later.
- Each language fixture compiles its source and runtime into the reference
  machine.
- Missing semantics, roots, mappings, and resources return engine errors.

Passing one language prover or one fixture cannot satisfy this gate.

A reused component summary also requires contextual trace equality. Until that
theorem exists, Stage 3 must keep the component in the monolithic model.

## 12. Current Rust gaps

The current Rust code does not meet this completion gate. Source inspection on
2026-09-04 found these explicit gaps:

- `adapters/rust/tools/mir-dump/src/rvalue.rs:42` maps `Repeat` and
  `ThreadLocalRef` to `Rvalue::Other`.
- `adapters/rust/tools/mir-dump/src/operand.rs:14,32` maps runtime checks and
  unparsed constants to `Operand::Other`.
- `adapters/rust/tools/mir-dump/src/terminator.rs:1-2` states that the
  conversion omits unwind edges.
- `adapters/rust/tools/mir-dump/src/terminator.rs:42` maps `Return`, `Resume`,
  `Abort`, and `Unreachable` to `End`.
- `adapters/rust/src/bound/domain.rs:18-25` reports several input domains as
  unknown.
- `adapters/rust/tools/mir-dump/src/main.rs:33-35` omits compiler runs for
  non-primary packages.

These gaps require engine errors under this design. They cannot produce
`PROVED` or `DISPROVED`.
