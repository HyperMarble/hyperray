# Rust root facts implementation plan

Status: PROPOSED ADDITIVE WORK. Planning text only, 2026-09-06.

**Goal:** Carry owned compiler argument facts through the same-build catalog without claiming valid machine roots.

**Architecture:** The existing compiler callback emits typed facts beside diagnostic ABI output. The public catalog reader and join retain those facts and explicit unresolved obligations.

**Tech stack:** Pinned Rust compiler APIs, Rust inventory producer, and Go catalog consumer.

**Specs:** `docs/design.md`, `docs/proof-machine.md`, `docs/semantic-coverage.md`, and `docs/plan-rust-same-build-inventory.md`.

## Ownership and dependencies

Fox owns the producer. Owl owns the consumer. Astra owns planning and review. Luna high writes implementation and tests.

This plan does not authorize changes to their frozen schema. Fox and Owl must agree on the additive schema and timing first. Complete or coordinate the current same-build fixes before this task starts. Do not change coverage admission, native execution, or machine thread code. Do not commit or close a gate under this planning assignment.

## Required roots versus present evidence

Every program entry and in-scope function instance requires exact standalone entry information, including instances outside program-start reachability. A normally valid function needs a nonempty entry set. An impossible precondition needs an accepted proof. Query selection cannot remove unrelated roots from coverage.

Sources: `docs/design.md:77-86`, `docs/proof-machine.md:135-145`, and `docs/semantic-coverage.md:90-98,159-160`. Boundary domains and complete machine state also matter: `docs/proof-machine.md:42-58,157-168`.

Current checks establish consistency between declarations and certificate rows:

- `coverage/root_catalog.go:28-51` checks state references or an impossible-proof reference.
- `coverage/root_path.go:5-25` requires exact root-to-state edges.
- `coverage/roots.go:45-50` compares mappings with declared entries.
- `coverage/unsupported.go:5-10` rejects the unsupported impossible proof recorded by the root catalog.

These checks do not derive Rust validity or compiler-to-machine entry states. They cannot independently establish the required root set.

## Available compiler facts

Source prefix `P` below means the installed pinned compiler source:

`/Users/hak/.rustup/toolchains/nightly-2026-08-21-aarch64-apple-darwin/lib/rustlib/rustc-src/rust/compiler/`

The driver currently stores ABI debug text at `adapters/rust/tools/mir-dump/src/inventory/callable.rs:15-20`. The consumer reduces root reports to instance IDs and reasons at `coverage/compilercatalog/join.go:81-84`.

Usable compiler APIs exist:

- `P/rustc_public/src/mir/mono.rs:76-90`: instantiated type, `fn_abi()`, and compiler symbol.
- `P/rustc_public/src/abi.rs:15-60,116-119`: ordered ABI arguments, return, convention, pass modes, and `Layout::shape()`.
- `P/rustc_public/src/abi.rs:282-339,395-437`: initialization class, primitive, width, and inclusive wrapping ranges.
- `P/rustc_public/src/ty/tys.rs:558-580`: structured source type classes.
- `P/rustc_public/src/target.rs:9-25`: target endianness and pointer width.
- `P/rustc_public/src/mir/mono.rs:174-183`: `requires_caller_location()`. Its implicit reference argument is absent from the MIR parameter list.

Compiler `Ty` and `Layout` handles are session data. Resolve their facts during the callback. Do not persist those handles as portable identities.

## Task 1: Add owned producer facts

**Files:** `adapters/rust/src/mir/inventory.rs` and `adapters/rust/tools/mir-dump/src/inventory/callable.rs`. Use existing inventory modules unless a split becomes necessary.

**Input:** The same concrete compiler instance used by the current successful build.

**Output:** Optional `root_facts: RootFacts` beside existing ABI diagnostics. Proposed `RootFacts.version` is 1. Absent facts in older records remain explicit unresolved evidence.

Proposed owned fields:

| Record | Fields |
|---|---|
| RootFacts | version, compiler target identity, endian, pointer_bits, convention, fixed_count, c_variadic, requires_caller_location, ordered args, return |
| Argument fact | source type class, size_bits, align_bytes, pass-mode tag, layout-ABI tag, scalar components when present, unresolved details |
| Scalar fact | Initialized or Union, primitive kind, width/signedness or address space, optional wrapping-range endpoints |

Use canonical decimal strings for `u128` endpoints. Preserve wraparound instead of sorting endpoints. Preserve non-scalar types and their actual tags. An ADT or reference with scalar layout remains an ADT or reference.

Pass-mode attributes remain explicitly opaque. `P/rustc_public/src/lib.rs:319-340` limits `Opaque` to diagnostic compiler data. `P/rustc_public/src/abi.rs:44-60` uses it for Direct, Pair, Cast, and Indirect details. Record unresolved attributes, not parsed debug strings or invented register assignments.

Capture the selected target from `tcx.sess.opts.target_triple` in the same callback. Sources: `P/rustc_session/src/config.rs:1459,2477-2486`. Compare it with manifest Target and the owned compiler arguments. For custom JSON targets, record the actual target-content identity or leave the case unsupported. `TargetTuple::tuple()` returns only a filename stem for that case: `P/rustc_target/src/spec/tuple.rs:110-118`.

- [ ] Agree on schema timing with Owl before edits.
- [ ] Add failing real-producer tests for the cases below.
- [ ] Emit owned facts without changing legacy extraction or claiming roots.

## Task 2: Retain facts through the public consumer

**Files:** `coverage/compilercatalog/types.go`, `read.go`, and `join.go`.

**APIs:** Keep public `Read(content []byte) (Artifact, error)` and `Join(artifact Artifact, build BuildManifest, objects []ObjectArtifact, elfContent []byte, maximumLoadedBytes uint64) (Report, error)`. Add owned facts to `Instance` and root report rows. Do not create a `ValidatedRoot` or call `coverage.Check`.

**Input identities:** Actual inventory bytes/digest, compilation ID, instance ID, manifest target, object bytes, and linked ELF bytes. `Join:22-36` already compares compilation identity and artifact hashes.

**Output:** One retained report row per producer instance, typed facts where available, and unresolved obligations. Compare compiler target facts with manifest target and supported ELF machine/class/data encoding. Width and endianness alone do not identify a target.

Reject malformed present facts, unknown fact versions, out-of-width endpoints, contradictory shape fields, and cross-artifact identities. Missing legacy facts create a missing-facts obligation. Hash consistency does not authenticate a fabricated compiler artifact against a malicious producer.

Primitive integer and bool argument values can be constructed from explicit boundary domains. This does not construct their complete standalone machine states. Preserve char, floats, ADTs, references, pointers, aggregates, variadics, and opaque lowering as unresolved cases in this first consumer.

- [ ] Add failing reader and join tests before implementation.
- [ ] Preserve owned facts and enforce structural consistency.
- [ ] Keep all current coverage and provenance obligations visible.

## Acceptance

Fox owns producer tests in `adapters/rust/tests/compiler_inventory_real.rs` and source fixtures. Owl owns consumer tests in `coverage/compilercatalog/read_test.go` and the public linked-build integration.

Use the same driver invocation for inventory and object. Link a real static RV64 Linux ELF, then call public Read and Join. The current test requests Darwin with linker `-r` at `compiler_inventory_real.rs:69-76`. That route does not satisfy this acceptance.

Required real cases:

- Distinct generic u8/u64 instances and an uncalled retained export.
- Bool, signed integers, usize target width, and lossless u128 endpoints.
- Char, NonZeroU8, references, and slices retained without false scalar-root admission.
- A retained `#[track_caller]` function with its implicit ABI argument.

Compare exact producer and consumer facts, order, identities, and obligations. Do not count rows alone.

Negative tests mutate one property of a real baseline: missing or unknown-version facts, malformed endpoints, out-of-width values, contradictory Union/range fields, changed instance/target, mixed digests, or a missing required caller-location field. A stripped or missing symbol retains unresolved entry mapping rather than deleting the root obligation. A false compiler assertion is not detectable merely by rehashing the same false artifact.

Exercise inclusive wrapping-range boundaries separately if real fixtures produce no wrapping range. Label those cases synthetic. Do not claim compiler-generated wraparound evidence without an observed fixture.

Char demonstrates a required limit: `P/rustc_ty_utils/src/layout.rs:421-425` gives range 0 through 0x10FFFF. `P/rustc_const_eval/src/interpret/validity.rs:874-881` also excludes surrogates 0xD800 through 0xDFFF. Layout ranges alone are not complete validity predicates.

Proposed new external integration entry point: `coverage/compilercatalog/root_facts_real_test.go:TestRootFactsRealLinkedBuild`. It must invoke the public build owner, then Read and Join. Missing tools are an explicit test error, not a silent skip. Do not replace Owl's current integration while its fixes are active.

After implementation and tool coordination, run these commands in sequence from the repository root:

```sh
(cd adapters/rust/tools/mir-dump && cargo test --offline)
cargo test --offline --manifest-path adapters/rust/Cargo.toml --test compiler_inventory_real
go test ./coverage/compilercatalog -run '^TestRootFactsRealLinkedBuild$' -count=1 -v
go test ./coverage/compilercatalog ./coverage
```

The driver path must name the newly built pinned producer. The integration log must show the named test and the actual compiler/linker commands. Record exits, artifact identities, and skips. No test or build ran during this planning audit.

## Non-goals and remaining proof obligations

This task does not produce valid machine roots or impossible-precondition proofs. Even scalar functions still need entry-PC evidence, exact ABI lowering, stack/return context, initialized globals, runtime/environment state, and boundary-backed domains.

References add allocation identities, initialization, bounds, alignment, lifetime/aliasing rules, permissions, and provenance. Constant allocation metadata is not a general valid-heap generator. Runtime roots also need the declared one-hart software-thread and Linux-user environment.

The current inventory iterates codegen units at `inventory/mod.rs:48-66`. This does not establish equality with every boundary-declared function instance or all linked runtime instances. Keep that scope obligation visible.

No symbol association or DWARF location proves operation equivalence, elimination, or valid roots. This plan does not narrow the full root requirement. Completion means only that real compiler facts reach their public consumer faithfully, with all missing proof obligations explicit.
