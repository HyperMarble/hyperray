# Explicit compiler build selection

## Contract

The extraction API must not enable every Cargo feature implicitly.
`extract::CompileRequest` supplies absolute tool paths, the crate directory,
the output directory, and an explicit `FeatureSelection`.
`extract::run` accepts this request instead of three paths.
This change updates the existing callers. No implicit all-feature wrapper remains.

Cargo receives `--no-default-features` when the request disables default features.
Cargo receives only the additional feature names in the request.
The shared `FeatureSelection` type also serves native Cargo preparation.
The returned compiler record retains the requested feature selection.
Cargo still validates feature names and feature compatibility.

Extraction remains a host `cargo check --all-targets` operation.
It uses the selected manifest's Cargo package selection, including workspace defaults.
This work does not add cross-compilation or enumerate every feature combination.
Features define different builds. One selected build does not prove every configuration.
Full executable provenance and semantic coverage remain separate obligations.

Each extraction creates a new compiler-wrapper path and collects only new dump paths.
Directory-read errors must return errors, not an empty successful inventory.
A successful Cargo exit without new compiler output must also return an error.
The source fixture files must remain unchanged.

## Acceptance

A real crate must contain mutually exclusive features and a default feature.
The same compiler connection must extract the default, alternate, and no-feature builds.
Conflicting and unknown features must remain compiler errors.
A repeated selection must produce fresh dumps, not reuse the earlier inventory.
The earlier live compiler extraction and rejection tests must still pass.
Ordinary request tests must reject missing tools and ambiguous feature lists.

The command contract comes from the installed `cargo check --help` output.
The compiler driver retains its `nightly-2026-08-21` toolchain pin.

## Measured result: 2026-09-05

All eight feature-selection cases passed through the public extraction API.
Five successful selections produced ten fresh compiler dumps.
The three rejected selections retained their expected compiler diagnostics.
The repeated default selection did not reuse a prior dump.
The source fixture files stayed byte-for-byte unchanged.

The missing-inventory test first produced real compiler dumps.
It then supplied a successful replacement tool that produced no output.
The adapter rejected that result despite the earlier dumps.
Both earlier live compiler integration tests also passed explicitly.
The retained log is `/tmp/hyperray-compiler-final.R7gc4m/compiler.log`.

The driver reported `rustc 1.100.0-nightly (8925ea358 2026-08-20)`.
Adapter tests, strict Clippy, and the Cargo preparation matrix passed.
The source audit covered twenty files. The largest file had 70 lines.
The longest function had 30 lines. The code-style skill set these source limits.
The completion-gate skill required real compiler output and rejection tests.

This repair does not close the full Rust coverage gate.
Arbitrary native signatures, automatic thread setup, and formal proof acceptance remain open.
