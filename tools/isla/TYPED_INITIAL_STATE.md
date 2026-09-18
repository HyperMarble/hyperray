# Typed initial-state source artifact

`patches/typed-initial-state-v1.patch` starts from upstream commit `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
It includes the earlier executable-memory changes. Do not apply those earlier patches first.
The upstream license and included notices remain applicable.

## Build procedure

In a fresh checkout of the pinned commit, apply the patch:

```sh
git apply --check /Volumes/Hak_SSD/hyperray/tools/isla/patches/typed-initial-state-v1.patch
git apply /Volumes/Hak_SSD/hyperray/tools/isla/patches/typed-initial-state-v1.patch
env CARGO_BUILD_JOBS=2 cargo test --locked --offline --workspace
env CARGO_BUILD_JOBS=2 cargo build --locked --offline --release --bin isla-axiomatic --bin isla-litmus-dump --bin isla-footprint
```

The offline commands require cached Cargo dependencies and installed native dependencies.
`measured-typed-initial-state.json` records the local build and dependency digests.
All exported source files matched a patched clean checkout.
The recorded executables came from the working source, not a second build in that clean checkout.

## Public checks

The environment variables in [README.md](README.md#integration-tests) select the tools and model inputs.
Point all three Isla executable variables at the new tools.
From the Hyperray root, invoke the typed-state tests:

```sh
go test -count=1 -timeout 10m -tags isla_integration ./machine/isla -run '^TestRealRust(TypedTrapState|RejectsWrongTypedState|TypedInputChangesResult)$' -v
```

`ProgramBoundary.InitialState` carries named values in the existing Isla syntax.
The program digest binds these values to both execution stages.
The tests include normal compiled code, explicit fault state, and invalid structured input.
The model declarations supply the register types. The transport has no Rust instruction rules.

The fault fixture uses the matching upstream notification callback and an explicit trap-entry stop boundary.
It does not supply a general operating-system handler or a trap-completion classification API.
Full coverage and the 100 MB target remain unproved.
