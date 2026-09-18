# Same-build Rust compiler inventory

Status: implementation plan. This connection does not establish full pipeline support.

## Scope clarification, 2026-09-07

The user defines coverage as support through the selected Rust compiler,
executable, ISA semantics, and solver pipeline. Finite acceptance tasks provide
evidence for that support. This does not require checking every execution of
every Rust program. Finish Rust acceptance first. Do not introduce unrelated
runtime features or select a task-specification language during this stage.

The user now authorizes small, reviewed commits. Preserve unrelated work and
commit one logical change with its measured acceptance evidence. Do not push.

## Purpose

The current MIR driver and executable tests use different compiler invocations. This stage connects compiler-generated instance bodies to the actual object build and linked ELF. The result exposes missing proof obligations rather than hiding them.

Source evidence comes from `docs/semantic-coverage.md`, `adapters/rust/tools/mir-dump/src/main.rs`, `instance.rs`, `block.rs`, and `machine/isla/rust_compile_real_test.go`. The existing statement converter removes non-Assign statements. The existing body catalog does not associate concrete instances with their bodies. Neither route provides complete provenance.

## Ownership

Astra owns this plan and independent review. Luna high writes production code and tests. Preserve legacy MIR schema 4 and extraction behavior. Use small reviewed commits. Do not push, change target semantics, or alter the existing coverage admission API.

Start with concrete modules. Split files only when readability requires it. Do not create a general compiler framework. Rust compiler-specific conversion stays in `adapters/rust`. Shared Go artifact reconciliation stays outside that adapter tree.

## Producer and build owner

Expose a public Rust build operation and a command-line caller. The request supplies source, tool paths, selected compiler flags, boundary artifact, linker inputs, and a fresh output directory. It never supplies successful exit codes or claimed artifact hashes.

The build operation invokes the pinned driver with object and dep-info output. An explicit inventory mode records concrete codegen instances from that same invocation. Legacy extraction retains its current behavior.

For each concrete `MonoItem::Fn`, use the pinned public bridge and `Instance::body()`, `fn_abi()`, and `mangled_name()`. Never synthesize a generic instance. Record static, global-assembly, intrinsic, and absent-body cases explicitly. The pinned compiler source documents these APIs in `rustc_public/src/mir/mono.rs:50-90`.

Retain every original statement position and terminator, including cleanup blocks. Preserve the owned compiler payload beside the operation index. Do not use the lossy legacy assignment converter. Compiler session handles are not stable artifact identities.

The callback occurs before codegen completes. Only the build owner can finalize a successful manifest, after successful compiler and linker exits. It hashes the actual sidecar, object, ELF, tools, and available dependency/linker inputs. It preserves failure diagnostics without a successful build result. It must reject stale outputs and mismatched builds.

## Identity and limits

Use a separate versioned sidecar. Input identities exclude output digests to avoid circular hashing. Instance identities include compilation identity and compiler symbol. Body identities include instance, MIR phase, and body digest. Operation identities retain original block and statement positions or the terminator position.

Record actual dep-info, explicit extern artifacts, selected sysroot, and compiler/linker arguments. Missing input closure remains an explicit obligation. Dep-info alone does not prove complete source or runtime closure.

ABI records expose compiler facts and unresolved root obligations. They do not establish reference validity, aliasing constraints, initialized memory, or valid entry states. Ordinary instance bodies come from optimized MIR. They do not prove preservation of pre-optimization operations.

## Public consumer

Add a concrete public artifact reader and join operation under `coverage/compilercatalog`. Every public signature type must be externally constructible. The consumer accepts actual referenced artifact bytes and rejects missing, stale, duplicate, inconsistent, or cross-build identities.

Compare indexed operations with all positions in retained bodies. Preserve every producer instance. Load the linked ELF through `machine.Load`. Report object and image symbols at their actual evidence strength. A common symbol name or function range is not an operation-to-instruction proof.

The report exposes unresolved source/runtime closure, pre-optimization preservation, roots, linker origins, and semantic mappings. It must not return `ValidatedCoverage`, fabricate mapped or eliminated dispositions, or invoke `coverage.Check` with a substitute fixture graph.

## Acceptance

The external integration path calls the public build owner, obtains a linked ELF, then calls the public reader and join operation. A syntax-only or `cargo check` result does not pass.

Required cases include distinct generic instantiations, an uncalled retained export, a compiler shim, repeat values, and static data. Add real compiler-stage cleanup and thread-local cases under explicitly supported profiles. Do not fabricate compiler enum values as their only evidence.

Each positive case retains every raw-body position and the relevant instance/root obligations. Actual sidecar, object, and ELF digests must agree with the build manifest. Full proof obligations remain unresolved.

Each negative case starts from an accepted baseline. Mutate one property: remove a non-Assign operation, duplicate an identity, change body ownership or target, mix builds, corrupt bytes, or force compilation/link failure. Require the exact named failure. Optimized-away and stripped symbols retain explicit unresolved obligations rather than disappearing.

Run adapter tests, driver tests, `go test ./coverage/compilercatalog ./coverage ./layout`, and the new real linked-build integration tests. Record exact commands, exits, logs, artifact identities, and skipped tests. Native Isla is not required for this compiler/linker/loader stage. Run expensive builds in sequence on internal storage.

## Completion boundary

This stage is complete only when the real public build and consumer path passes every listed case and independent review accepts it. This stage alone does not establish support through ISA semantics and the solver. Track those connections separately against the selected compiler and target capabilities. Producer authenticity remains a stated trust boundary.
