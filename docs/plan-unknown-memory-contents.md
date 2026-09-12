# Unknown memory contents

Status: parser landed. Unknown contents do not yet reach the solver.

## Problem

One `backing` entry does two jobs at once:

1. It declares a region readable, and writable when its permission says so.
   `sequential_setup.rs` builds the readable and writable ranges from the
   backing list, and the ARM memory policy rejects any access outside them.
2. It supplies the region's exact initial bytes. `run_litmus.rs` calls
   `initialize_section` with those bytes.

A caller who wants unknown inputs must therefore delete the entry, which also
deletes the permission. The run then fails with `Bad read ARM read backing`,
which is correct behavior for an undeclared region, and useless for the caller.

Measured on the real Leaky ReLU fixture: removing the 32 input bytes at
0x400000 produced that rejection in 3 seconds.

## Goal

Prove one property for every possible input, rather than one property for one
fixed input. This is the difference between checking 6 cases and checking all
2^256 combinations of eight 32-bit lanes.

## What already works

Any address outside the initialized set is already symbolic. `SequentialMemory`
declares one unconstrained array and stores only the initialized bytes into it,
so an unstored address reads as an unknown value. No new solver work is needed.

`^name` already names an observation's value before the program ran, so a rule
can relate the final value to the initial one without naming a literal.

## Proposed contract

`bytes` becomes optional. A backing entry without it declares the region and
leaves its contents unknown:

```toml
{ address = "0x400000", permission = "RW", length = "0x20" }
```

An entry keeps its current meaning when `bytes` is present, so every existing
program text stays valid. Exactly one of `bytes` and `length` must appear:
`bytes` gives the contents and implies the extent, `length` gives the extent
and leaves the contents unknown.

## Changes

1. `arm_memory/table.rs`: `Backing` carries an extent and optional contents.
2. `arm64_memory.rs`: accept either field set; reject both together and
   neither, naming which entry is wrong.
3. `run_litmus.rs`: call `initialize_section` only for an entry with contents.
4. `sequential_setup.rs`: build readable and writable ranges from the extent,
   which is now independent of whether contents exist.
5. `page_table/setup.rs`: use the extent for collision and coverage checks.

The Go boundary gains an optional length, and the renderer emits `length`
instead of `bytes` when the caller declares no contents.

## Tests

Each states its expected result before it runs.

1. A backing entry with neither `bytes` nor `length` is rejected, naming it.
2. An entry with both is rejected, naming it.
3. An entry with `length` alone parses, and its region is readable.
4. A program that copies unknown input to output proves with `*out = ^in`.
5. The Leaky ReLU rule proves over unknown lanes.
6. The same rule with the shift changed to four is disproved, and the
   counterexample names a negative lane.

Test 6 is the negative control. Without it, test 5 could pass vacuously,
which is exactly what happened when the concrete all-positive inputs made the
shift branch unreachable.

## Measured blocker

The parser change landed and 204 tests pass, but a proof over unknown inputs is
not yet sound. `initial_memory::definition` writes one function that returns a
stored byte for each initialized address and `#x00` for every other address.

An uninitialized address therefore reads as a concrete zero, not as an unknown
value. Measured on the real fixture: with the input region declared by length
alone, the correct rule and a rule with the shift changed from three to four
both returned `forbidden`. A negative control that cannot fail proves nothing.

The initial memory array already exists as one unconstrained symbolic array in
`SequentialMemory`, but its symbol is private and has no accessor. The next
change makes that symbol reachable and reads the initial byte from the array,
so an undeclared address stays unknown instead of becoming zero.

Until then, a caller must declare `bytes` and prove one input at a time.

## Out of scope

Boundary derivation from the binary. That is a separate change, and the repo
deliberately refuses to infer a boundary from symbols.
