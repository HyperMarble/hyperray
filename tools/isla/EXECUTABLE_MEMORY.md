# Executable and memory source artifact

`patches/executable-memory-v1.patch` includes the candidate-output changes and the executable-memory connection.
It starts from upstream commit `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
Do not apply the earlier candidate patch first.
The upstream license and the included sequential-memory notice remain applicable.

## Build procedure

In a fresh checkout of the pinned commit, apply the patch:

```sh
git apply --check /Volumes/Hak_SSD/hyperray/tools/isla/patches/executable-memory-v1.patch
git apply /Volumes/Hak_SSD/hyperray/tools/isla/patches/executable-memory-v1.patch
env CARGO_BUILD_JOBS=2 cargo test --locked --offline --workspace
env CARGO_BUILD_JOBS=2 cargo build --locked --offline --release --bin isla-axiomatic --bin isla-litmus-dump --bin isla-footprint
```

The offline commands require the cached Cargo dependencies and installed native dependencies.
The measured native solver is `/opt/homebrew/opt/z3/lib/libz3.5.1.dylib`.
`measured-executable-memory.json` records its digest, the toolchain, and the rebuilt executable digests.
This record describes one local build. It does not promise identical executable bytes on other machines.

## Public integration

The environment variables in [README.md](README.md#integration-tests) select the tools and model inputs.
Point all three Isla executable variables at the rebuilt tools.
Keep the original `riscv64.toml` configuration for the declared profile.

From the Hyperray root, invoke the function-table regression:

```sh
go test -count=1 -timeout 10m -tags isla_integration ./machine/isla -run '^TestRealRustSymbolicFunctionTable$' -v
```

The test compiles Rust code and invokes the public SDK.
It requires both indirect-call targets and rejects outputs outside the declared set.
Two other queries require a concrete counterexample for each permitted output.

Hyperray supplies `-I '__monomorphize_reads = true'` to both sequential program stages.
It supplies the unchanged configuration through `--footprint-config` for isolated instruction analysis.
The default axiomatic profile retains its existing arguments.
This is a program-independent connection to the existing Sail address partition.

The test declares 2048 MB for the tool memory limit, not 100 MB.
Passing tests do not establish full translation equality or full program coverage.
Upstream compiler warnings remain.
