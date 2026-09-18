# Plan: complete finite-state proof architecture

Depth: tree 5   Mode: orchestrated
Budget note: This is a proof subsystem and five language front ends. Each leaf
must finish from a fresh context and pass its own observable gates.

## Contract

The proof claim is exact: for a closed finite boundary, Hyperray checks every
allowed execution of the model. `PROVED` means no execution violates the named
requirement. `DISPROVED` includes the requirement, state, path, and cause. A
model, coverage, tool, or resource failure is an engine error, not a verdict.

Semantic coverage, reachability, and verdict are separate values. Every
compiler-inventoried operation must map through compiler output and machine
instructions to one or more transitions. Each proof query names its exact root
set. A function has exact valid entry states or a proof that none exist.
Unreachable operations remain in the coverage map. Only a complete coverage
certificate can enter the proof engine.

Coverage uses generated catalogs and exact set equality. The compiler supplies
the operation catalog. The executable loader supplies the instruction catalog.
Pinned Sail supplies the instruction-semantic catalog. External operations use
declared finite contracts. The circuit lowerer supplies guarded regions.

The coverage checker proves both directions. Each required case has an exact
model effect, and each model effect has one declared origin. It also validates
all provenance edges and artifact digests. One missing or extra case is an
engine error. A test suite, sample, count, or handwritten pattern list cannot
replace this equality.

The model reports undefined behavior as a named safety violation unless the
language gives it defined semantics. A proof never removes undefined behavior
from its execution set without an explicit requirement. Nonterminal deadlocks
are not normal termination. A temporal requirement uses a finite generalized
Buchi monitor with exact fairness rules.

The reference route is language-neutral:

```text
source + finite boundary
  -> compiler inventory + executable image + provenance
  -> finite machine semantics + finite external contracts
  -> complete semantic-coverage certificate
  -> fixed-point and accepting-cycle proof
  -> PROVED or DISPROVED
```

Rust, C, C++, and Go compile into the same machine model. The Python route
includes the pinned interpreter and its bytecode. Compiler-level adapters can
accelerate analysis, but the machine route remains the final coverage authority.
Language-specific provers are accelerators only. They never establish
coverage. An accelerator result is accepted only after equivalence with the
reference transition model is checked.

Component reuse needs an exact summary proof. The proof relates interface
traces, hidden state, interference, termination, and fairness to the component
transition model. A changed dependency hash invalidates the summary.

Each external call has a finite contract for its inputs, results, visible
effects, and state changes. Hyperray does not model the operating-system
implementation behind that contract. A missing contract returns an engine
error.

The machine-level theorem names the exact executable, Sail semantics, and
external contracts. Compiler-level results are accepted only after equivalence
with this reference route.

The first executable kernel uses public Go packages. `model.Model` is a small
explicit transition-graph oracle. `coverage.Check` validates explicit-graph
coverage. `proof.Check` calculates exact reference results for small models.
These packages do not represent the production machine state by enumeration.

The production route uses a symbolic sequential circuit. Circuit coverage maps
semantic cases to guarded equations and proves that their union equals the
transition relation. An IC3 result requires an independently validated
inductive invariant. A bounded search requires a proved completeness threshold
before it can return `PROVED`.

The v1 graph contains states and directed transitions with unique identifiers.
Each state can contain named values for an exact witness. Each transition
names its source state and target state.

The v1 inventory contains functions, roots, compiler operations, synthetic
operations, and environment operations. Each root maps to one or more exact
entry states. Each operation maps through compiler output, an instruction, a
semantic rule, and one or more graph transitions. Every graph transition has
this evidence. This first version rejects elimination records that it cannot
prove.

One proof query names one requirement and a nonempty set of inventory roots.
The coverage gate still measures all inventory roots and operations. Query
selection changes reachability only. It never changes the coverage count.

The v1 proof kernel accepts safety and total-termination requirements. A safety
requirement can name bad states and bad transitions. Total termination requires
every maximal query execution to enter its terminal-state set.

`coverage.Check` returns an opaque `ValidatedCoverage` capability. Only the
coverage package can create a valid capability. It stores the canonical model,
coverage report, and exact root mappings. `proof.Check` accepts this capability
and query root identifiers. It never accepts a caller-created coverage report
or caller-selected state roots. A zero capability returns an engine error.

A result contains exact coverage counts and reachable identifiers. Safety,
nonterminal-deadlock, and nonterminal-cycle disproofs have different tagged
witnesses. Every witness names its root, full path, state values, transitions,
requirement, and cause.

Names use complete words. Library failures return errors. No production code
panics, exits, skips work, invents a limit, or converts an engine error into a
logic verdict. Each code leaf owns only the files listed below.

## Tree

- 1 Complete finite-state proof architecture .......... `gates/root.md`
  - 1.1 Freeze the proof contract ...................... `gates/node-design.md`
    - 1.1.1 Rewrite the architecture documents ......... `gates/leaf-design.md`
    - 1.1.2 Define versioned artifact schemas .......... `gates/leaf-schemas.md`
  - 1.2 Build the language-neutral reference kernel .... `gates/node-kernel.md`
    - 1.2.1 Implement finite transition models ......... `gates/leaf-model.md`
    - 1.2.2 Implement semantic coverage validation ..... `gates/leaf-coverage.md`
    - 1.2.3 Implement exhaustive proof and traces ...... `gates/leaf-proof.md`
    - 1.2.4 Expose the JSON CLI path .................... `gates/leaf-cli.md`
  - 1.3 Build the universal machine route .............. `gates/node-machine.md`
    - 1.3.1 Define semantic IR and contracts ............ `gates/leaf-semantic-ir.md`
    - 1.3.2 Prove circuit lowering and coverage ......... `gates/leaf-circuit.md`
    - 1.3.3 Add complete and accelerated solvers ........ `gates/leaf-solvers.md`
    - 1.3.4 Join the trusted same-program Isla slice ..... `gates/leaf-isla-same-program.md`
  - 1.4 Connect source languages ....................... `gates/node-languages.md`
    - 1.4.1 Connect Rust compiler artifacts ............. `gates/leaf-rust.md`
    - 1.4.2 Connect C and C++ compiler artifacts ........ `gates/leaf-c-family.md`
    - 1.4.3 Connect Go compiler artifacts ............... `gates/leaf-go.md`
    - 1.4.4 Connect Python and its interpreter .......... `gates/leaf-python.md`
  - 1.5 Prove integration and composition .............. `gates/node-integration.md`
    - 1.5.1 Prove source-to-result fixtures ............. `gates/leaf-end-to-end.md`
    - 1.5.2 Prove exact component composition ........... `gates/leaf-composition.md`

## File ownership

### Active coverage completion work (2026-09-05)

The full root objective remains unchanged. These tasks remove concrete gaps.
Passing one task does not establish full bounded-program coverage.

- Trace-tree correctness owns new semantic-tree files, the existing
  semantic_instruction.go and semantic_threads.go, related tests,
  docs/isla-semantic-tree.md, and gates/leaf-isla-semantic-tree.md.
  The parser must preserve parent state at each fork and account for events
  regardless of line layout. It must use the pinned writer grammar.
- Compiler coverage stress owns new Rust machine fixtures and new
  rust_patterns_real_test.go files, plus gates/leaf-rust-patterns.md.
  It must use the existing compiler helper and public proof API unchanged.
  Failures remain evidence of gaps, not permission to exclude a construct.
- The parent owns program-boundary and loaded-memory integration. It also
  audits both tasks against the full coverage contract and repeats their tests.
- The initialized-memory agent owns new upstream initialized-memory modules,
  changes to isla-lib/src/memory.rs and isla-axiomatic/src/smt_events.rs,
  their unit tests, and docs/isla-initial-memory-backend.md. It must coordinate
  changes to upstream lib.rs with the parent. The parent owns litmus input
  parsing, setup, tool capabilities, and the Hyperray bridge.

These tasks use the existing public contracts. No agent can mark the full
root complete from a fixture result or change another task's owned files.

Early sequential memory integration uses `docs/isla-sequential-memory.md` and
`gates/leaf-isla-sequential-memory.md`. The initialized-memory agent owns new
upstream sequential-memory modules, callback error propagation in memory.rs,
and the public write-options observer in smt.rs. The parent owns the optional
profile, setup validation, public Go selection, and end-to-end integration.
The semantic-tree agent audits mixed-width candidate semantics without edits.
Existing concurrent semantics remain unchanged. Full coverage stays open.

- 1.1.1 owns `docs/design.md`, `docs/stage3.md`, `docs/stage4.md`,
  `docs/proof-machine.md`, `docs/semantic-coverage.md`, and
  `docs/instruction-translation.md`.
- 1.1.2 owns `schemas/`.
- 1.2.1 owns `model/`.
- 1.2.2 owns `coverage/`.
- 1.2.3 owns `proof/`.
- 1.2.4 owns new files under `cmd/hyperray/` and `testdata/proof/`.
- 1.3.1 owns new files under `semantic/`.
- 1.3.2 owns new files under `circuit/`.
- The executable loader owns new files under `machine/`.
- 1.4.1 owns `adapters/rust/`, including
  `adapters/rust/tools/mir-dump/`.
- 1.4.2 owns `adapters/c/` and `adapters/cpp/`. These directories do not
  share production source files.
- 1.4.3 owns `adapters/go/`.
- 1.4.4 owns `adapters/python/`.
- 1.5 owns `integration/` and `testdata/languages/`.

## Status log

- 2026-09-05: Shared memory-read uniqueness resolves the stored return address.
  All five Rust call and storage cases passed both queries without source changes.
  The recursion fixture declares four instruction visits, independent of two host workers.
  A two-visit request returned an explicit error without a verdict.
  The invalid-stack, mixed-width, and symbolic-input regressions also passed.
  The upstream library passed 99 unit tests and three documentation tests.
  The complete upstream workspace tests passed. The Go package race tests passed.
  All fourteen listed real-tool tests passed across three batches, without omissions.
  `gates/leaf-rust-patterns.md` records the five-case result.
  General multiple-target coverage, translation equality, environment semantics,
  reproducible tool patches, and the 100 MB product target remain open.

- 2026-09-05: Two shared boundary checks rejected a symbolic PC after the last
  instruction. The parser defers the address requirement until an instruction
  event. Isla performs its existing explicit exit before instruction counting.
  The unchanged generic-call and dynamic-dispatch fixtures then passed both
  proof queries. Recursion still requires symbolic-address resolution during
  execution. `gates/leaf-isla-symbolic-pc.md` retains the open obligations.

- 2026-09-05: The unchanged stored-pointer fixture passed both real proof
  queries after the sequential-memory rebuild. Mixed-width and stack-value
  fixtures also passed both queries. Invalid stack access remained an error
  with no verdict. Its test no longer rejects added source-location details.
  Generic calls, dynamic dispatch, and recursion stop at the shared symbolic
  instruction-address connection. The next unit is
  `docs/isla-symbolic-pc.md` and `gates/leaf-isla-symbolic-pc.md`.
  These results do not establish full coverage or the 100 MB product target.

- 2026-09-05: Lean accepted nine event-interface theorems and rejected two
  false claims. The proofs preserve generic choices and effects. They also
  establish a missing upper endpoint in the imported range primitive.
  `docs/sail-event-route.md` records the integration limits. Separately, all
  twelve existing sequential-memory tests passed, including the corrected
  ordinary-store contract. Two existing upstream compiler warnings remain.
  The full coverage goal remains open.

- 2026-09-05: Compiler extraction no longer enables every Cargo feature.
  The public request declares the feature selection and retains it in the result.
  Eight real selection cases passed and produced ten fresh dumps across five
  successful builds. Conflicts, unknown features, and missing fresh inventories
  remained errors. `docs/compiler-selection.md` records the acceptance evidence.
  This is implementation progress, not completion of the full coverage goal.

- 2026-09-05: The active continuation made implementation progress.
  `execution/nativecheck.Run` adds diagnostic result validation and automatic replay.
  All seventeen native cases and four Cargo cases passed through this API.
  Replay and boundary rejection retained the original search evidence.
  `docs/native-result.md` records the limits and the retained execution log.
  This API grants no proof capability. Arbitrary signatures, automatic thread
  setup, provenance, and formal proof acceptance remain open.

- 2026-09-05: Cargo preparation passed four feature-selection cases and three
  real build-error cases. The earlier seventeen native cases also passed.
  Cargo compiled the workspace, dependency, renamed libraries, and build script.
  The fixture files stayed unchanged. The largest runtime peak sum was
  34,848,768 bytes, excluding compilation. `docs/cargo-preparation.md` records
  the scope and retained log. Arbitrary signatures, automatic thread setup,
  and formal result validation remain open. No commit or push occurred.

- 2026-09-05: `docs/native-preparation.md` records automatic preparation for the
  experimental native Rust scalar interface. Seventeen declared cases passed
  preparation and observation, including a bug replay and explicit partial search.
  The runtime peak sum was 33,177,600 bytes. Universal preparation, Cargo builds,
  automatic thread setup, and the full proof-result path remain open.

- 2026-09-05: After the native SPIN and Loom experiments, execution integration
  starts with `docs/execution-observer.md`. This separate diagnostic path retains
  the proof evidence rule. An ordinary successful exit cannot produce `PROVED`.
  `gates/leaf-execution-observer.md` records the SDK and CLI acceptance checks.

- 2026-09-05: The user approved Lean for the translation proof work.
  `docs/lean-proof-route.md` preserves the full coverage contract and records
  the generated model's fixed-choice and external-axiom obligations.
  The imported-memory proof leaf owns only `proof/sail_memory/lean/`.
  The parent owns its replay, dependency pins, audit, and integration.
  `gates/leaf-lean-memory-proof.md` records the intermediate proof obligations.
- 2026-09-05: The complete imported RV64 Lean model built successfully.
  The parent replayed three universal upstream-memory theorems and rejected
  a changed read-after-write claim. The full machine-step axiom audit still
  reports platform and floating-point assumptions. No full-coverage gate closed.

- 2026-09-04: The plan fixes the proof claim, error rule, route, and ownership.
- 2026-09-04: The baseline has 27 passing Rust tests and no strict Clippy error.
- 2026-09-04: The baseline Go test and Rust format checks pass.
- 2026-09-04: The parent reran the design gates. All 6 gates pass.
- 2026-09-04: The parent validated all 6 schemas. All 7 schema gates pass.
- 2026-09-04: The frozen proof-contract branch passes all 3 integration gates.
- 2026-09-04: The first model checks passed. A defect pass reopened the leaf for stable validation order and canonical fixtures.
- 2026-09-04: The first coverage checks passed, but an independent audit found missing evidence catalogs and exact-root checks. The leaf is open again.
- 2026-09-04: The paper audit keeps BTOR2 as an intermediate format. It rejects bounded search as proof without a proved completion result.
- 2026-09-04: Coverage now requires generated catalogs, bidirectional set equality, exact provenance, and circuit-region equivalence.
- 2026-09-04: Rotor and Rotor-Rust were cloned for local source measurement. They are implementation seeds, not coverage authorities.
- 2026-09-04: The explicit model leaf passed all 8 gates and an independent audit. Its 30 tests give 100.0% statement coverage.
- 2026-09-04: The parent reran every executable design gate. The design leaf passes all 8 gates.
- 2026-09-04: A local Rotor-Rust audit found 94 decodable instruction IDs in its largest profile, but only 46 explicit RV64IM transition cases. Compressed instructions lack complete instruction-specific transition cases, so Rotor-Rust cannot establish coverage.
- 2026-09-04: A Z3 miter covered every pair of two 8-bit inputs. Equal step functions returned UNSAT; a `<` to `<=` mutation returned SAT with `balance = cost = 32`.
- 2026-09-04: The circuit leaf now treats translation as its main proof obligation. The executable supplies instruction identities, pinned Sail supplies reference meaning, typed semantic IR supplies shared equations, and Lean must prove Sail-to-IR and IR-to-circuit equality.
- 2026-09-04: The universal machine route is the final coverage authority. External effects use finite contracts instead of a complete operating-system implementation. Compiler-level routes are accelerators.
- 2026-09-04: The target comparison keeps little-endian RVA20U64 and a finite Linux-user EEI for production. A single-threaded WASIp1 profile can become a later experimental route.
- 2026-09-04: The coverage API must return an opaque validated capability. A caller-created report or accepted flag can never authorize Stage 4.
- 2026-09-04: The official RV64 integer slice passed for all 22 operations in five families and all values in their finite input types.
- 2026-09-04: One-bit mutations failed for all five proved families and returned finite counterexamples.
- 2026-09-04: The harness takes all four operation catalogs from the pinned official source. Lean proves that all four catalogs are exhaustive.
- 2026-09-04: Sail generated the larger `I_insts` Lean project, but Lean rejected `Defs.lean` because generated identifiers and dependent parameters are invalid. This output is not coverage evidence.
- 2026-09-04: The typed Sail tree produced equal declaration, decoder, and execution ownership sets for all 23 RV64 base-I instruction families, including legal illegal-instruction traps. This catalog proves ownership, not instruction meaning.
- 2026-09-04: The RV64 family-local meaning result covers five families. This result is 5/23, or 21.7%, and does not prove the integrated machine step.
- 2026-09-04: The Rust adapter now reads the schema version before required schema fields. All Rust adapter tests, Rustfmt, and strict Clippy pass.
- 2026-09-04: The production machine route now uses Sail JIB as its semantic IR. Handwritten instruction-family proofs remain reference fixtures only.
- 2026-09-04: Each source language owns a separate adapter directory. The
  shared machine, coverage, circuit, and proof engines remain language-neutral.
- 2026-09-04: The pinned upstream Sail backend lowered the complete RV64
  base-I profile to 1,735 JIB definitions. The exhaustive grammar census found
  14 instruction kinds and recorded `undefined` instead of omitting it.
- 2026-09-04: The same traversal assigned stable identifiers to 167,953 JIB
  constructor origins. The artifact digest is
  `a0df439fb9be58269ab215e17101c991d44afb65c6000cc0f8bddb5e6fd33818`.
- 2026-09-04: The JIB-to-circuit catalog validator requires exact origin and
  region equality. It rejects missing, extra, duplicate, and implicit mappings.
- 2026-09-04: The direct Sail C++ route generated a 2,968,951-byte base-I
  model. ESBMC parsed it and rejected platform types from the larger profile.
- 2026-09-04: The production candidate now reuses the official Sail
  SystemVerilog backend. Hyperray supplies root selection, width obligations,
  provenance, synthesis, and the translation proof.
- 2026-09-04: The same-program Isla verifier returned `PROVED` for the safe
  ADDI program and `DISPROVED` with `x5 = 3` for the unsafe program.
- 2026-09-04: The verifier binds both operations to one program request. Its
  failure tests and the complete Isla package measured 100.0% statement coverage.
- 2026-09-06: Both sequential program stages reuse Sail's existing address
  partition with separate footprint configuration. All fifteen listed real-tool
  tests passed in one command. This fixture result is not full semantic coverage.
- 2026-09-06: The executable-memory source artifact passed all five acceptance
  gates. All 49 source files matched a clean clone, 140 native tests passed,
  and the three rebuilt tools passed the public function-table queries.
- 2026-09-06: A separate diagnostic reused the upstream notification callback
  and retained architectural fault writes at an explicit trap-entry boundary.
  The public trap contract remains open. No production fault behavior changed.
- 2026-09-06: The public program API carries typed initial state through the
  existing Isla value parser. Both execution stages require its capability.
  The compiler-built fault-address queries and normal typed-input queries
  passed. Invalid structured state returned an error without a verdict.
- 2026-09-06: All eighteen listed real-tool tests passed across the full
  seventeen-test command and the additional typed-input command. The native
  workspace passed 143 tests. The typed-state source patch reconstructs all
  54 exported files from the pinned upstream commit. Full coverage remains open.
- 2026-09-06: Solver-backed model-call safety passed all four leaf gates.
  The public SDK distinguishes a forbidden trap from a normal return and
  exposes the selected call observations. No source-program rule was added.
  All twenty real-tool tests passed in 478.830 seconds. The native workspace
  passed 148 tests. The cumulative source patch reconstructs 60 matching files.
  Structured trap state, the complete exit contract, full semantic coverage,
  and the 100 MB target remain open. No commits or pushes were made.
- 2026-09-06: Structured register assertions passed all four leaf gates.
  The final register-field build passed 154 native tests and 23 real-tool tests
  in 558.796 seconds. Its source patch reconstructs 71 matching files.
  The SDK exposes the architectural fault cause and the feasible trap call.
- 2026-09-06: The instruction-visit guard now uses the existing native
  mechanism in both whole-program tools. Both results record the request limit.
  The latest recursive end-to-end test stops in the trace stage without a verdict.
  Its first test expected the old solver text. The corrected test requires
  the actual PCLimitReached trace error and passed. Full regressions continue.
- 2026-09-06: The user prioritizes end-to-end prototype tests before memory
  optimization. The 100 MB target covers all Hyperray processes, including
  child tools. The surrounding Anchor target is 200–300 MB. Neither memory
  target is a measured result. No optimization or Anchor integration is part
  of the current test pass.
- 2026-09-06: The SDK prototype passed the buggy-code-to-repair test.
  Both compiler-built programs used the same requirement. The buggy stack
  update returned DISPROVED with result 12. The repair returned PROVED for
  required result 13. The test driver supplied the repair, not an autonomous agent.
  All 24 listed real-tool cases have passing results across three commands.
  The initial full command recorded one obsolete error-text assertion failure.
  Its corrected rerun passed. The native workspace passed 158 tests.
  The source patch reconstructs 73 matching files. Memory optimization,
  Anchor integration, and full semantic coverage remain separate work.
