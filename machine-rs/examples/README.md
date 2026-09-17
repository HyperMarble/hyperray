# loadtest

Splits the fixed cost every `isla-axiomatic` process pays before it proves
anything.

Measured on the AArch64 model (25 MB IR), one-instruction program:

| stage | cost |
|---|---|
| parse the model (`src/axiomatic.rs` does this 3x) | 0.59s each |
| `initialize_architecture` (done 2x) | 0.62s each |
| Isla's own reported proof work | 0.62s |
| unattributed (process start, Z3 load, litmus parse) | ~0.5s |

Total wall clock 4.75s, of which 3.61s is loading the same model repeatedly.

The repetition is deliberate upstream. `src/axiomatic.rs:307` says:

    // Huge hack, just load an entirely separate copy of the architecture
    // for footprint analysis

`footprint_operate.go:24` spawns one process per instruction, so an
N-instruction proof pays the 3.6s N+1 times.
