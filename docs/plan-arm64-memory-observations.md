# ARM64 Memory Observations

## Goal

Expose checked, named memory observations from the public Go ARM64 builder. Render the observations as native metadata for the unchanged `*name` expression syntax.

## Scope

This change owns the Go API, Go validation, Go rendering, immutable `Program` metadata, public evidence, and Go tests. It does not change native protocol code, Sail code, compiler code, or fixture source files.

The related native transport design is [Sequential terminal-memory transport](plan-sequential-terminal-memory.md). That design consumes the rendered `[[memory_observations]]` tables and resolves observation names before final assertion evaluation.

## Public contract

`MemoryObservation` contains:

- `Name string`
- `Address uint64`
- `Bytes uint32`

`ARM64ProgramBoundary.MemoryObservations []MemoryObservation` supplies the declarations. An absent or empty slice emits no observation tables and preserves the existing behavior.

Each observation is metadata only. It does not initialize memory, add a load, add a store, or change instruction behavior. The existing `*name` expression grammar remains unchanged.

## Validation

Names use the existing `identifier` grammar: the first byte is an ASCII letter. Later bytes are ASCII letters, digits, or underscore. Names must be printable single-line values. The reserved names are `arch`, `name`, `symbolic`, `memory_profile`, `code_ranges`, `memory_observations`, `arm64_memory`, and `thread`.

The Go builder also rejects a name that collides with the selected native namespace: a `symbolic_addrs` key, a `[types]` or `sizeof` key, an exact `register_renames` alias, or the encoded name of an `isa.default_registers` key or `isa.reset_registers` root location. The fixed ARM64 profile includes names such as `VBAR_EL1`, `CPACR_EL1`, and `InGuardedPage`, as well as unpadded aliases such as `R0`, `X0`, and `W0`. Padded spellings such as `R00` are not rejected unless the native profile configures them.

The builder rejects an observation when:

- its name duplicates another observation;
- its name matches a reserved name or exact native namespace name;
- `Bytes` is not 1, 2, 4, or 8;
- `Address + Bytes` overflows `uint64`;
- the complete range is not backed by loaded image bytes or by one explicit caller memory backing.

A loaded image range is checked from the image’s actual loaded bytes. Explicit caller backing is checked from the validated `ARM64MemoryInput` copy and must use `MemoryRead` or `MemoryReadWrite`. The builder does not alter symbolic base or stride values to fit an observation.

## Ownership and evidence

`BuildARM64Program` validates observations before rendering. It stores an independent copy in `Program`. Public accessors return independent copies. `Program.Evidence` includes the same observation metadata so callers can inspect the accepted contract without reading generated text.

## Rendering

Each declaration renders as one canonical lower-case hexadecimal address and an integer byte width:

```toml
[[memory_observations]]
name = "lane0"
address = "0x400000"
bytes = 4
```

The renderer sorts declarations by address, then by name for equal addresses. This makes output independent of declaration order. Overlapping observation ranges are valid because observations have no execution effects.

## Test gates

Public tests run against the checked-in real Mach-O fixture. They cover:

1. the public type and boundary field;
2. exact canonical rendering and declaration-order independence;
3. absent metadata behavior;
4. loaded-image and explicit-backing coverage;
5. malformed names, duplicates, collisions, unsupported widths, and overflow;
6. copy isolation through the boundary, `Program`, accessor, and evidence;
7. the genuine Leaky ReLU fixture across all six `cases.json` rows, with eight lane observations and count, stack-pointer, and program-counter assertions. The negative control changes only the property.

Ordinary Go tests and `go vet` are allowed. Tagged acceptance runs and Cargo builds are not run by this worker.
