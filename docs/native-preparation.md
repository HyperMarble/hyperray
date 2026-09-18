# Automatic native checker preparation

## Scope

The Rust adapter prepares the native SPIN checker from a request.
The request names Rust source, an entry function, separate requirement source,
and a requirement function. The compiler validates their signatures.
The subject accepts `u64` and returns `u64`.
The requirement accepts the input and output as `u64` and returns `bool`.
This first interface does not accept arbitrary function signatures.

The request declares an inclusive input interval, search depth, hash-table size,
state-memory limit, runtime timeout, output limit, and runtime memory budget.
The preparer must not substitute a smaller input interval or infer requirements.
The input interval can include the full `u64` range.
This range support does not guarantee practical search completion.

The compiler supplies program behavior. SPIN supplies search behavior.
Generated connection code calls the subject and its separate requirement.
No source-pattern, instruction, or program-name dispatch belongs in the preparer.
The generated search chooses bits of an offset in the declared interval.
Padding offsets outside the interval do not call the subject.
Observation counts are diagnostic call counts, not independent coverage proofs.

## Existing boundary

This is the deterministic native route from the earlier experiment.
The subject must terminate without persistent state, external input, threads,
or undefined behavior. The preparer does not establish these conditions.
Heap allocation success remains an assumption of this route.
The caller supplies trusted source and tools. This is not a security sandbox.
Automatic thread instrumentation remains separate work.

The preparer compiles standalone source files with rustc and links the generated checker.
The optional Cargo path builds library packages through Cargo.
Its contract and limits are in `docs/cargo-preparation.md`.
Separate source modules can resolve through their original source paths.
Tool paths are explicit. The result records tool version output.
Source and dependency provenance certification remains separate work.

## Public path

`hyperray_rust::prepare::build` accepts a public request and returns artifact paths.
The `hyperray-native-prepare` binary accepts the same request as JSON on stdin.
Its JSON result includes a request for `hyperray observe` and a replay executable.
Preparation does not run the subject and cannot produce a correctness verdict.

The output directory must not exist. An existing directory returns an error.
Compiler errors retain their stage and full log path. Failed builds remain on
disk for diagnosis and cannot return a prepared result.
Build tools operate synchronously. Build time and memory are separate from the
100 MB runtime gate. Build cancellation and sandbox isolation remain unfinished.

## Acceptance

The same preparer must build the six earlier native subjects at two optimization
levels. New fixtures must change entry names, requirement names, and input ranges.
A changed solution must expose its counterexample through the generated checker.
Replay must reproduce the counterexample. Bad signatures and invalid bounds must
return explicit errors. The observer must measure each declared integration case.
No passing research artifact can change.

## Measured result: 2026-09-05

All seventeen preparation cases passed through the public JSON binary and SDK.
Fifteen correct cases finished their searches. One broken case produced a
counterexample and reproduced it through replay. One full-width case deliberately
stopped at its depth limit and retained the partial-search diagnostic.
That last case does not establish exhaustive coverage of all `u64` inputs.

The largest worker-plus-observer peak was 33,177,600 bytes, or 33.18 MB.
This measurement excludes preparation and compiler memory.
Existing-directory rejection, compiler signature errors, and missing output
artifacts also passed their integration tests.

The five Rust request tests, adapter tests, strict Clippy, Go regression tests,
and Go static analysis passed. Generated C passed Clang analysis without
user-code warnings. Clang reported 612 suppressed warnings in non-user code.
Rustfmt and Clang-format accepted the generated connection files.

The Linus-style skill constrained source files and exposed error decisions.
The completion-gate skill required the whole matrix, error cases, and replay.
The measured source audit found at most 72 lines per file and 32 lines per function.
The matrix log is `/tmp/hyperray-preparation.4hT7vZ/matrix.log`.
An example request and its generated checker remain in the same temporary directory.

The preparation command is `cargo run --bin hyperray-native-prepare` from
`adapters/rust`. It reads the request from stdin and writes the result to stdout.
The result's `execution` object is the input for `hyperray observe`.
The result also remains in `prepared.json` in the requested output directory.

Arbitrary function signatures, thread instrumentation,
build cancellation, sandbox isolation, and formal result validation remain open.
This milestone does not close universal preparation or full coverage.
