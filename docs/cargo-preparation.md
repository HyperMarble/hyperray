# Cargo preparation for native Rust checkers

## Contract

An optional `cargo` object selects Cargo preparation in the existing request.
Without this object, the existing source-file path remains unchanged.
In Cargo mode, each `source` names a package's `Cargo.toml` file.
The subject and requirement still name public functions with the existing scalar signatures.
Virtual workspace manifests are not package manifests.

The Cargo object supplies an absolute Cargo executable path and explicit feature
selections for the subject and requirement. Each selection contains
`default_features` and `features`. The preparer must not infer feature choices.

The preparer generates a separate binding package in the new output directory.
Cargo adds the two path dependencies and builds the binding as a static library.
Cargo owns dependency resolution, feature activation, build scripts, and linking.
The existing SPIN and native-observer paths remain unchanged.

The generated package has its own workspace and lockfile. Cargo resolves this
wrapper offline, then builds it with `--locked --offline`.
The input projects and their lockfiles must remain unchanged in the fixture tests.
The generated resolution is not a claim of equivalence with a production lockfile.
Project-specific Cargo configuration is not automatically copied into the wrapper.
Missing offline dependencies and incompatible configuration return build errors.

The preparer reads Cargo's JSON artifact messages, not guessed filenames.
Only the generated manifest's `binding` static library can supply the native archive.
A missing, duplicate, malformed, or unsuccessful build result returns an error.
Compiler output and diagnostics remain separate files.

## Boundary

This path builds selected library packages, not every workspace target or binary.
The native route's deterministic, terminating, single-thread assumptions remain unchanged.
The preparer does not establish these assumptions or claim full language coverage.
Build scripts execute during preparation. Inputs and tools must remain trusted.
Build cancellation, isolation, and compilation-memory limits remain unfinished.
The 100 MB gate still applies to the checker and observer at runtime only.

## Acceptance

Real offline fixtures must exercise a workspace, renamed library targets,
path dependencies, a build script, and explicit feature changes.
An incorrect function must produce a counterexample and replay it.
Unknown features, bad signatures, absent artifacts, and existing directories
must remain errors. The earlier source-file matrix must still pass.

The local `cargo add --help`, `cargo build --help`, and Cargo's installed
`reference/external-tools.html` define the command and artifact contracts.
The ledger is `gates/leaf-cargo-preparation.md`.

## Request extension

The public Rust types are `prepare::CargoOptions` and `prepare::FeatureSelection`.
The JSON request accepts this additional field:

```json
{
  "cargo": {
    "executable": "/absolute/path/to/cargo",
    "subject": {"default_features": false, "features": ["boost"]},
    "requirement": {"default_features": false, "features": []}
  }
}
```

This fragment is not a full request. All existing request fields remain required.
The subject and requirement source paths must name their package manifests.
The compiler validates the public function paths through the generated binding.
The result retains the existing execution request and replay path.

## Measured result: 2026-09-05

All four Cargo cases passed through the public preparation binary and execution SDK.
The cases selected no default features, default features, an explicit feature,
and an incorrect implementation. The incorrect implementation produced a counterexample at input 23.
Replay reproduced that counterexample. The correct cases each recorded eleven observations.
These observation counts are not independent coverage proofs.

The fixture includes workspace inheritance, renamed libraries, a custom source path,
a path dependency, and a build script. Its files and directory inventory stayed unchanged.
Unknown features, a wrong signature, and a virtual workspace manifest returned errors.
Failed builds retained diagnostic logs and did not publish `prepared.json`.

All seventeen earlier native cases also passed, including the deliberately partial search.
The Cargo cases measured a peak sum of 12,255,232 bytes.
The earlier native cases measured a peak sum of 34,848,768 bytes.
Both sums include the worker and observer, but exclude compilation.
The combined log is `/tmp/hyperray-cargo.ZuzT99/matrix.log`.

Four artifact tests and eight public request tests passed.
All Rust adapter tests, strict Clippy, Go tests, and Go static analysis passed.
The source audit covered 32 files. The largest file had 65 lines.
The longest function had 35 lines. The Linus-style skill set these size limits.
The completion-gate skill required the full fixture matrix and explicit error cases.

This result closes Cargo preparation for this interface, not universal program coverage.
Arbitrary function signatures, automatic thread setup, and formal result validation remain open.
