# Typed initial machine state

Trap registers expose a missing input connection. The public program boundary accepts integer registers but cannot express structured machine state.
The existing Isla value parser already accepts structured values, Boolean values, vectors, and enumeration members.
Hyperray must reuse that parser, not add Rust instruction rules or a second value language.

`ProgramBoundary.InitialState` supplies named values in the existing Isla value syntax.
The generated program records these values in a top-level `initial_state` table.
The program digest thus binds the initial state to both proof stages.
These assignments apply to each modeled thread before its instructions.
The existing integer `InitialRegisters` remain supported.
A register cannot occur in both declarations. Duplicate names and empty values are errors.

Both program tools must accept the required `--typed-initial-state` capability flag.
A tool without that capability must return an error, not silently omit the table.
Standalone instruction analysis retains its original state.

The upstream parser supplies the values. The model declarations supply the register names and types.
Before execution, the adapter must reject unknown registers and values that do not match those declarations.
Structured fields must match the declared field set and field types.
Vector lengths, element types, enumeration membership, and bitvector widths must match the declarations.
The assignments must not modify other registers or model definitions.

The first public regression uses structured trap-vector state and a bitvector fault-address requirement.
It must retain the architectural fault writes and pass both the declared requirement and its changed requirement.
This input connection does not by itself classify trap completion or support structured final assertions.
Those parts remain in `gates/leaf-isla-trap-boundary.md`.

## Measured public result

The native initial-state tests passed. The tests exercised the existing value parser and the declared fixture types.
The public wrong-type test returned the `mtvec` type error without a verdict in 32.21 seconds.
The public fault-address test passed both queries in 67.17 seconds.
The result contained `PROVED` for the declared fault address and `DISPROVED` with `0:mtval=#xfffffffffffffff8;` for the changed requirement.
The compiler evidence retained `addi sp, sp, -32` and `sd ra, 24(sp)`.
With initial `sp = 0`, those instructions place the store at the declared fault address.

The new build completed in 3 minutes 13 seconds in `/tmp/hyperray-initial-state.BtoOF9/target/`.
The earlier measured executables and source patches remain unchanged.
The seventeen-test real-tool repeat passed in 798.368 seconds.
The additional ordinary typed-input test passed in 36.94 seconds.
A name-by-name comparison found eighteen listed tests and eighteen passed tests, with no missing, failed, or skipped tests.
The ordinary inputs produced counterexamples `32` and `16`, as the fixture's division requires.
These results do not establish full coverage or the public trap-completion contract.

The native workspace passed 143 tests, with no failed or ignored tests.
The new native modules had no Clippy diagnostics with the configured shape limits.
The new native files have at most 71 lines. Existing upstream warnings remain.
The Go formatter and integration-tagged `go vet` passed.

`tools/isla/patches/typed-initial-state-v1.patch` reconstructs the source from the pinned upstream commit.
The export retained the final context line after its first application check failed.
The repaired patch applied to a fresh checkout. All 54 exported files matched the working source byte for byte.
`tools/isla/measured-typed-initial-state.json` records the local build. All ten recorded file digests matched fresh measurements.
