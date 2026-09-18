# Concrete candidate output

This patch targets Isla commit
`7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
The upstream source is `https://github.com/rems-project/isla`.
The upstream license remains applicable to the modified source.

The patch adds `--candidate-status` to Herd output.
Each state row identifies its candidate as `allowed` or `forbidden`.
The solver query requests distinct symbolic final-register values.
The existing model reader supplies the concrete register values.
Unsupported values cause Hyperray to reject the result.

The patch does not alter instruction semantics or solver constraints.
The final state is evidence of an outcome, not a replayable input trace.
All three tools report the version suffix `/candidate-status-v1`.
Hyperray also requires the expected executable and input digests.
A version suffix alone does not establish trusted provenance.
`measured-release.json` records the local executable and input digests.

## Build

In a clean checkout of the pinned commit, apply the patch:

```sh
git apply --check /Volumes/Hak_SSD/hyperray/tools/isla/patches/candidate-status-v1.patch
git apply /Volumes/Hak_SSD/hyperray/tools/isla/patches/candidate-status-v1.patch
cargo test -p isla-axiomatic final_value::tests
cargo build --release --bin isla-axiomatic --bin isla-litmus-dump --bin isla-footprint
```

The build uses the upstream dependencies and local Rust toolchain.
The upstream README specifies the required native tools.
The local build retains pre-existing upstream compiler warnings.

## Integration tests

Supply paths through these environment variables:

- `HYPERRAY_RUSTC`
- `HYPERRAY_RUST_LINKER`
- `HYPERRAY_ISLA`
- `HYPERRAY_ISLA_DUMP`
- `HYPERRAY_ISLA_FOOTPRINT`
- `HYPERRAY_SAIL_IR`
- `HYPERRAY_ISLA_CONFIG`
- `HYPERRAY_MEMORY_MODEL`.

From the Hyperray root, invoke the real-tool tests:

```sh
go test -count=1 -tags isla_integration ./machine/isla -run 'TestRealRust(SymbolicBranchInput|ExecutablePrograms)' -v
```

The tests require the installed `riscv64gc-unknown-linux-gnu` Rust target.
They invoke the compiler and linker, then use the public verification API.
Missing environment variables cause an error, not a skipped test.
The gate record is `gates/leaf-symbolic-rust-input.md`.
