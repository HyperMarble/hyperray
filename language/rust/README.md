# Rust adapter

Builds a Cargo project the project's own way, records every file the build
made and what made it that way, and hands the files to the loader.

It never looks inside the code. The compiler turns every Rust shape into
machine code; the adapter picks that machine code up. So "does the adapter
handle generics?" means "does the compiler build generics into a program,
and does the adapter hand that program over?"

## Coverage of Rust shapes

Proof: the Rust team's own test suite (`tests/ui` at commit `50d54098`,
18 Sept 2026), every test that builds into a program on an ARM Mac, built
with nightly Rust (`db8f076d2`, 3 Oct 2026). 4,328 programs.

Every program the compiler built, the adapter handed over: 4,194 of 4,194.

Counts per shape are "handed over / compiler built". A shape is checked
when the adapter missed none of the programs the compiler built for it.

- [x] Structs, enums, unions, tuples: 175 / 175
- [x] Arrays, slices, strings, boxes, unsized values: 139 / 139
- [x] Layout and representation (`repr`, packed, never type, transmute): 75 / 75
- [x] Generics (const generics, associated types, `impl Trait`, inference): 311 / 311
- [x] Traits (bounds, coherence, specialization, `dyn`, coercion, casts): 562 / 562
- [x] Functions, methods, closures, function pointers: 279 / 279
- [x] Macros, attributes, derives, proc macros: 309 / 309
- [x] Async, coroutines, pinning: 171 / 171
- [x] Unsafe, raw pointers, inline assembly, C calls: 167 / 167
- [x] Statics, thread locals, allocators, `no_std`: 55 / 55
- [x] Control flow, patterns, expressions: 385 / 385
- [x] Ownership, borrowing, lifetimes, drop: 316 / 316
- [x] Const evaluation: 264 / 264
- [x] Numbers, iterators, collections, std: 275 / 275
- [x] Modules, crates, editions, linking: 208 / 208
- [x] Threads, processes, panics, runtime: 200 / 200
- [x] Code generation settings, SIMD, LTO, debug info: 303 / 303

The other 134 programs never reached the adapter:

- 95 could not be set up by the lab's wrapper (helper libraries, revision
  names, placeholders). Lab work, not the compiler and not the adapter.
- 38 the compiler refused: 20 use nightly features removed after the tests
  were written, 7 need checkers Rust does not ship for Mac, 6 are marked
  broken by the Rust team, 1 is x86 only, 1 crashed the compiler, 3 other.
- 1 is unsupported, see below.

## Unsupported

- Test-mode programs (what `cargo test` builds). The adapter builds the
  real program only. A patch that changes only a `#[test]` function has no
  machine code to judge yet.

## What the record holds

- every file the build made: kind, hash, features, settings it was compiled
  with (opt level, debug info, debug assertions, overflow checks), and its
  debug-info file when there is one
- the compiler: version, host, hash
- what was asked: profile and features; the lock file; Cargo's settings
  files, deepest first
- the environment variables that change a build (secret values hidden)
- C code linked in by build scripts, and the C compiler and SDK used
- the OS build
