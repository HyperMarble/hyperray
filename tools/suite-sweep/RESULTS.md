# Suite sweep results

188 of 188 assertions PROVED, over 1652 real ARM64 instructions.

Each assertion is taken from Rust's own integer test suite
(`library/coretests/tests/num/int_macros.rs`), compiled by rustc to a Mach-O
binary, and proved against the ARM Sail semantics through Isla and Z3. The
verdict is about the machine code, not about the Rust source.

## Coverage

| assertions | Rust test |
|---:|---|
| 113 | test_pow |
| 13 | test_rotate |
| 9 | test_saturating_abs |
| 9 | test_saturating_neg |
| 9 | test_next_multiple_of |
| 7 | test_le |
| 7 | test_be |
| 5 | test_swap_bytes |
| 4 | test_signum |
| 4 | test_div_floor |
| 4 | test_div_ceil |
| 3 | test_abs |
| 1 | test_rem_euclid |

## What PROVED means here

For each assertion the solver shows that no execution of the real
instructions can end with `R0` holding anything other than the expected
value, and with `_PC` at the return address. `black_box` keeps the work in
the machine code, so the proof examines instructions rather than a constant
the compiler folded.

## What it does not cover

- One fixed input per assertion. These are the suite's own constant cases,
  not a proof over all inputs. Proofs over unknown register inputs work and
  are demonstrated elsewhere, for example `leading_zeros` over all 2^64.
- 64-bit signed integers only. The suite instantiates its macro for each
  integer type; this sweep resolves `$T` to `i64`.
- The assertion checks `R0` and `_PC`. It does not state that the code writes
  nothing else.

## Reproducing

```
cd /Volumes/Hak_SSD/hyperray/tools/suite-sweep && ./sweep.sh
```

`check.py` runs first and asks rustc whether every extracted assertion holds.
If extraction is wrong the sweep stops instead of producing verdicts about
code the suite never wrote. Results are saved after each case, so an
interrupted run resumes where it stopped.

`./sweep.sh --refresh` re-reads the suite after a rust-lang update and keeps
a result only when the compiled code is byte-identical.

## Negative control

A verdict that cannot fail is worthless. Flipping one bit of the expected
value on `(-1).rem_euclid(i64::MIN)`, from `0x7fffffffffffffff` to
`0x7ffffffffffffffe`, changes the verdict from PROVED to DISPROVED.
