# Rust adapter

Status: **DESIGN — 2026-09-01**

The Rust instantiation of the Hyperray language-adapter protocol
(`lang-adpaters/PROTOCOL.md`). It states which external tool fills which stage,
what each one was measured to do, and what remains unbuilt.

Every measurement in this document was produced on 2026-09-01 against
noodles-296 (14-file Rust patch, async, generic, `tokio`). Nothing here is
cited from memory. Where a claim is untested, it says so.

---

## 1. What the adapter is for

Hyperray proves four things about one fixed bounded task
(`docs/specs/finalarchitecture.md` §1):

1. the reference solution implements the required logic over the whole bounded
   scope;
2. no incorrect behavior passes the tests (no false positives);
3. no correct behavior is rejected by the tests (no false negatives);
4. the reference passes the verifier over the whole bounded scope.

`spec.md` is the axiom. The adapter's job is to supply the **language
intelligence** the generation core must not guess: the finite domains, the
typed transitions, the loop bounds, and the machine-checkable evidence that a
row is real.

The adapter never reads `instruction.md`, `test.patch`, grader files, or
task-authored tests. That wall is the protocol's, and it is the same wall as
the spec skill's phase-1 freeze.

---

## 2. Stage map

```
        solution.patch + base checkout
                    |
   [1] EXTRACT      |  Charon / rustdoc JSON / -Zdump-mir
                    v
             extraction manifest  (PROTOCOL fmt 1 or graph fmt 2)
                    |
   [2] BOUND        |  RustHorn -> LoopInvGen        (loop invariants)
                    v
             manifest + Invariants column filled
                    |
   [3] PROVE        |  Kani (autoharness, contracts, stubs)
                    v
             T(C)=true, safety obligations discharged
                    |
   [4] ADEQUACY     |  cargo-mutants / IVC           (is the spec strong enough)
                    v
             surviving mutants -> missing spec rows
                    |
   [5] PLAN         |  NOT BUILT
                    v
             plan.json: which rows get a test, which get a sentence
                    |
   [6] EMIT         |  NOT BUILT
                    v
             test.patch + instruction.md, from one source
```

Stages 1-4 are existing tools, measured below. Stages 5-6 do not exist for
Rust and are the adapter's actual engineering work.

---

## 3. Stage 1 — EXTRACT

### 3.1 Primary: Charon

`github.com/AeneasVerif/charon` (arXiv 2410.18042). Emits one JSON
(`.llbc`) holding `type_decls`, `fun_decls`, `global_decls`, `trait_decls`,
`trait_impls`, plus cleaned function bodies with constants reconstructed and
overflow checks made explicit.

Its stated purpose is exactly this adapter's problem:

> centralize the efforts of extracting information from rustc internals and
> turning them into a uniform and usable shape

Alternatives with the same output shape, if Charon's Rust pin ever conflicts:
`runtimeverification/stable-mir-json`, `GaloisInc/mir-json`.

**Do not hand-roll a rustc driver.** Three teams already maintain one.

### 3.2 Fallback: two raw compiler dumps

Verified working on this machine when Charon is unavailable.

**Public surface**
```sh
RUSTDOCFLAGS="-Z unstable-options --output-format json" \
  cargo +nightly doc -p <crate> --features '<f>' --no-deps
# -> target/doc/<crate>.json
```
Measured: 537 items, 832 KB for `noodles-util`.

Mechanically derived from types alone: **156 obligations** —
Result 72, Option 26, `enum Format` 24, usize 12, bool 8, `enum Index` 6,
`enum Record` 3, Vec 3, `enum CompressionMethod` 2.

Type → domain table (this is the `Parameters:` line of a spec row):

| type | finite domain values |
|---|---|
| `uN` / `iN` | min, max, one past each (overflow witness) |
| `bool` | true, false |
| `Option<T>` | `None`, `Some(T)` |
| `Result<T,E>` | Ok arm, Err arm |
| `Vec<T>` | empty, one, many |
| `str` / `String` | empty, non-UTF8 |
| `enum E` | one value per variant |

Blind spots of rustdoc JSON: `impl Trait` (opaque), generic parameters
(unbounded), and **constants inside function bodies** — rustdoc exports no
bodies. Charon does not have this blind spot; the second dump exists for the
fallback path.

**Body values and compiler-inserted obligations**
```sh
cargo +nightly rustc -p <crate> --features '<f>' --lib -- \
  -Zdump-mir='<fn filter>' -Zdump-mir-dir=/tmp/mirdump
# read only *.SimplifyCfg-final.after.mir (~145 files per fn otherwise)
```
Measured on the 4 detection functions: 2033 files, 14 bodies at final stage.

Recovered what rustdoc could not see:
- constants: `8192:usize`, `2:usize`, `6:usize`, `0:u8`
- **3× `assert(..., "attempt to compute `{} - {}`, which would overflow")`** —
  rustc marks its own proof obligations; one of these is a real defect (§5)
- 55 `switchInt` sites with exact arm lists → finite branch domains

Grep targets: `assert(`, `SubWithOverflow`, `AddWithOverflow`, `switchInt`,
`const \d+_(usize|u8|i64|…)`.

### 3.3 What extraction feeds

| manifest field (PROTOCOL) | source |
|---|---|
| `operations[].domains` | type table (3.2) + enum variants |
| domain `source expression` | Charon `fun_decls` signatures |
| `graph.seeds` (fmt 2) | typed value domains from the type table |
| `graph.transitions` | public fns + ownership mode from Charon |
| exclusions | `switchInt` arms that are provably unreachable |
| constants in row outcomes | MIR body dump |

---

## 4. Stage 2 — BOUND (loop invariants)

A `while` with a data-dependent guard has no unwind bound readable from the
source. Two ways out, both older than the tools that ship them.

**Prior art**: Karr 1976 (affine relations); Cousot & Halbwachs 1978
(inequalities, abstract interpretation); Rodríguez-Carbonell & Kapur 2004 —
a **complete** algorithm for polynomial equalities terminating in at most
`2m+1` iterations, producing the *strongest* inductive invariant.
Boundary: Monniaux Problem (JACM 2025) — existence of a separating invariant
is undecidable for full polyhedra, decidable in polynomial time for a single
affine loop.

**Tool used: LoopInvGen** (`SaswatPadhi/LoopInvGen`, MIT, Docker image
`padhi/loopinvgen`). Input is a SyGuS invariant problem: state variables,
`pre_fun`, `trans_fun`, `post_fun`.

### Measured — both loops in the real solution patch

`solution.patch` contains exactly two loops in production code
(`indexed_reader/builder.rs`); the remaining 40 are in the test file.

| loop | site | invariant found | time |
|---|---|---|---|
| `detect` | builder.rs:232 | `0 <= plen <= 65536` | 0s |
| `read_at_least` | builder.rs:258 | `0 <= dlen <= 65536` | 5s |

Warm-up cases, same tool, same session:

| case | invariant found | time |
|---|---|---|
| Kani book countdown (`u64::MAX` unwinds) | `x >= 1` | 1s |
| two coupled counters | **`2x = y`** + `x>=100 → y=200` | 1s |
| buffer with 8192 step / 65536 cap | `len <= cap` | 0s |
| running sum | `i+s >= 0`, `i >= 0` | 1s |

The `2x = y` result matters: it is an *equality between variables*, outside
the "conjunctions of linear inequalities" that Kani's own
`--synthesize-loop-contracts` is documented to search. Karr's 1976 class.

### An empty model reports `false`

A first attempt at `read_at_least` returned `false` — no state satisfies the
transition relation. That was a modelling error (read modelled as exactly 8192
with `want <= 6`, so the body can never run twice), and it is also a true fact
about the code: for `want` of 2 or 6, `read_at_least` iterates at most once.

**Rule**: a `false` invariant is a translation bug, never a proof. Re-derive
the transition relation before proceeding.

### Where the transition relation comes from

Hand-written today. It must not stay that way.

`RustHorn` (Matsushita et al.) translates Rust MIR into constrained Horn
clauses, exploiting ownership to eliminate the heap; the translation is
proved sound and complete for the core language, and their implementation
"translates the MIR of a Rust program into CHCs quite straightforwardly."

Kani also builds this relation internally on every run
(`MIR → GOTO → SSA → formula`). Either source removes the manual step. This
is stage-1 work, not a separate stage.

Output lands in the spec's `Invariants` column.

---

## 5. Stage 3 — PROVE (Kani / CBMC)

```sh
cargo install --locked kani-verifier && kani setup      # ~2 min, pulls CBMC
cargo kani autoharness -Z autoharness -Z unstable-options \
  -p <crate> --features '<f>' --include-pattern <substr> --harness-timeout 60s
cargo kani autoharness ... --list          # dry run: eligible vs skipped + reason
```

`-Z autoharness` implies `-Z function-contracts` and `-Z loop-contracts`.
Defaults: 60 s per harness, `--default-unwind 20`. Flags are
`--include-pattern` / `--exclude-pattern`.

### 5.1 The whole patch compiles

All 14 patch files compiled into GOTO in **7.6 s**. Async, generics, trait
objects and `tokio` were not blockers. Compiling is not verifying; the
distinction is load-bearing and must be reported separately.

### 5.2 Everything proved, measured

| target | result | time |
|---|---|---|
| `poll_read` — every prefix/position/buffer | pass, 580 checks | 4s |
| `start_seek` `Start` — every offset | pass, 447 checks | 2s |
| `start_seek` `End` — every offset | pass, 447 checks | 1s |
| `poll_complete` | pass, 408 checks | 1s |
| `ReplayReader::new` | pass, 177 checks | 1s |
| `read_more` — **real `async fn`** via `kani::block_on` | pass, 770 checks | 1.7s |
| `scratch_len` — **by contract** | pass, 70 checks | 0.04s |
| `Builder::default` ×4 crates | 4 pass / 4 blocked | 7s |
| **`start_seek` `Current`** | **FAIL — real defect** | 0.4s |

### 5.3 The defect

```
builder.rs:322
  let unread_prefix_len = self.prefix.len() - self.position;
  SeekFrom::Current(offset - unread_prefix_len as i64)

Failed Checks: attempt to subtract with overflow
Counterexample: prefix_len=4, position=0, offset=-9223372036854775808
```

36 hand-written integration tests, a full `cargo-mutants` run, and a human
reviewer all missed it. Kani found it in 0.88 s without executing the code.
Its concrete-playback mode emits a runnable `#[test]` reproducing it
(`-Z concrete-playback --concrete-playback=print`).

The same obligation appears in the raw MIR dump as an `assert(...)`. Stage 1
can therefore *predict* which functions carry arithmetic obligations before
stage 3 runs.

### 5.4 The bound decides the answer

Same function, three bounds:

| bound on `prefix_len` | verdict | time |
|---|---|---|
| `== 0` | SUCCESSFUL | 0.31s |
| `<= 4` (author's invention) | FAILED | 0.41s |
| `<= MAX_DETECTION_PREFIX_LEN` (read from the code) | FAILED | 0.40s |

**Rule**: every bound is a constant already present in the patch, cited like
a spec row's `Evidence` cell. A bound the author invents can hide the defect
and produce a clean pass that means nothing — a false negative in the sense
of `finalarchitecture.md` §1.3.

### 5.5 Harnesses are generated, not authored

`kani autoharness` writes the `#[kani::proof]` internally; no source change.
Verified by deleting all six hand-written harnesses and re-running: same
results.

Skip reasons reported across the 4 crates: 2954 filtered by pattern,
**989 "Generic Function"**.

### 5.6 Generics: entry points only

The Kani book:

> If some caller of `foo` is eligible for an automatic harness, then a
> **monomorphized version of `foo` may still be reachable** during verification.

Confirmed. One concrete caller with primitive arguments — **no `kani::` calls,
no assertions** — reaches the generic body:

```rust
#[cfg(kani)]
fn concrete_seek_entry(prefix_len: u8, position: u8, offset: i64) {
    let p = (prefix_len % 5) as usize;
    let pos = (position as usize) % (p + 1);
    let mut r = ReplayReader { prefix: vec![0u8; p], position: pos,
                               inner: Cursor::new(Vec::<u8>::new()) };
    let _ = Pin::new(&mut r).start_seek(SeekFrom::Current(offset));
}
```
Result: same defect, builder.rs:322, **0.39 s**, zero verification code
written by hand. Sizes are bounded with `%` on a primitive argument, so no
`kani::assume` is needed either.

What remains is a **type choice** per generic parameter, derivable from the
trait bounds Charon already reports:
`R: AsyncRead + AsyncSeek + Unpin` → `Cursor<Vec<u8>>`. The adapter owns a
bounds→witness-type table; the graph manifest (PROTOCOL fmt 2) is where it
belongs, since `graph.seeds` already carries exact typed expressions.

The verify-rust-std effort has no better answer today:

> The result is a plethora of near-identical harnesses where only the types
> are different... this is the approach currently used in the verify-std
> effort, due to a lack of alternatives.

For a human that is drudgery; for a generator it is a loop.

### 5.7 Async works

`kani::block_on` drives a real `async fn`. `read_more` — the actual async
function from the patch — verified, 770 checks, 1.7 s. Requires `-Z async-lib`.

Contracts do **not** apply to `async fn`; the macro rewrite breaks `await`
(`error[E0728]`). Fix: hoist the decision into a small sync function. That is
the user's own authorship rule — *a decision never returned as a value cannot
be tested directly* — and `scratch_len` is the instance:

```rust
#[kani::requires(dst_len <= MAX_DETECTION_PREFIX_LEN)]
#[kani::ensures(|r: &usize| *r <= 8192 && *r + dst_len <= MAX_DETECTION_PREFIX_LEN)]
fn scratch_len(dst_len: usize) -> usize { … }
```
Proved for every `dst_len` in 0..=65536 in **0.04 s**. Those two numbers are
the 8192 ceiling and the 65536 cumulative bound — two spec rows, discharged
by proof rather than by sampled tests.

### 5.8 Contracts and stubbing (the scaling mechanism)

Two modes. **Enforce**: assume `requires`, run the body, assert `ensures` and
the `modifies` frame. **Replace**: at each call, assert `requires`, havoc the
frame, assume `ensures` — body never unrolled. Cost stops growing with call
depth.

Kani refuses a `stub_verified` whose contract has no passing
`proof_for_contract` harness, so the mechanism cannot be cheated.

**Soundness rule, and it is decidable, not a judgement call:**

> If the replacement produces a **subset** of the outputs, that is an
> under-approximation, which causes **unsoundness**. If the replacement
> produces a **superset**, that is an over-approximation, and crucially
> always sound.

Contracts are over-approximations by construction, hence always sound. Raw
`#[kani::stub]` is not, and every use needs the superset argument recorded —
this is a spec-level `Free:` decision, not an implementation detail.

**External crates**: the *double stub*. Stub the foreign function to a local
wrapper that immediately calls it, attach the contract to the wrapper, then
`stub_verified` the wrapper. Per the Kani blog this "does not pose a threat
to soundness because the potentially unsound stub effectively replaces the
function with itself."

### 5.9 Failure modes, with the measured cause

**Timeout is not "the dependency is big."** It is unresolved function
pointers (Kani #1767): CBMC replaces an indirect call with *every*
signature-matching candidate and explores all of them. `Display`/`write!` on
an error path is the classic trap.

Measured: `detect` timed out at 120 s. The tool named the loops it could not
finish, and that list **is** the stub list:

```
zlib_rs::crc32::braid::crc32_words_inner
zlib_rs::crc32::braid::crc32_naive_inner
flate2::gz::GzHeaderParser::parse
flate2::gz::read_to_nul
```

CRC32 and gzip header parsing — none of it in `solution.patch`.

Escalation order:
1. `#[kani::stub(orig, replacement)]`
2. `-Z function-contracts` + `stub_verified` (sound; prefer this)
3. `--solver kissat` (CBMC #7013: a hang became 2 s)
4. per-loop `--cbmc-args --unwindset <label>:<n>` (`--show-loops` for labels)

**Unsupported, seen in this session:**
- `CCRandomGenerateBytes` — any `HashMap::default()` / `RandomState` pulls in
  the macOS RNG. All 4 "failures" in the crate sweep were this, not defects.
  Keep std hash maps out of harness setup.
- closures as entry points (#3832); pointers; recursive `Arbitrary` derivation
- non-overridden default trait methods (#4588)
- a polymorphic `proof_for_contract` target admits **one** monomorphization
  per harness (RFC 0009) — CBMC checks one contract at a time

**Portfolio note.** Kani is not the only backend. verify-rust-std has
approved goto-transcoder (→ ESBMC, adds concurrency), VeriFast (separation
logic, unbounded, linked structures), and Flux (refinement types, lightweight,
infers loop invariants). Their finding: *"No one-size-fits-all approach"*, and
Kani solved 6 of 9 accepted challenges because it is easiest, not strongest.
Portfolio construction is measured research (CoVeriTeam: up to 3× faster
wall-time, 30-60% less energy) with one documented hazard — a parallel
portfolio that takes the first answer inherits the fastest *wrong* answer, so
a validation step is mandatory. Selection should key off stage-1 features
(has loops / generic / unbounded data / concurrent), the same way Nacpa
selects portfolios from extracted program features.

---

## 6. Stage 4 — ADEQUACY (is the spec strong enough)

This is the false-positive direction of `finalarchitecture.md` §1.2, and it
is the one place mutation belongs.

**Mutate the code, re-run the proof — not the tests.**

- proof still passes on a mutant → the mutant is **live** → the spec is too
  weak; a wrong implementation could pass. Missing row.
- proof fails → mutant **killed** → that spec row is doing work.

Published as *mutation model checking* / *mutation proving* (mCoq for Coq
projects). The numbers are sobering: two case studies' specifications killed
only **40% and 60%** of mutants; an industrial 1250-line SCADE cruise-control
killed **39%** while every line was already marked covered.

Cheaper relative: **Inductive Validity Cores** — ask the solver which model
elements the proof actually needed. Reported overhead **24%** over
model-checking alone versus **2369%** for mutation. IVC is coarser (line
granularity); mutation sees inside a line. Run IVC first, mutation on what it
flags.

Boundary, stated in the spec skill and restated here so the adapter never
oversteps it:

> Coverage, PICT, mutation testing, fuzzing, property-based testing... may
> find useful witnesses. **None can produce `VERIFIED`.**

Only the four Semantic-IR queries produce a verdict:

```text
EXISTS x,o: C(x,o) AND NOT R(x,o)                = UNSAT
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))       = UNSAT
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)     = UNSAT
T(C)                                             = true
```

Mutation supplies counterexamples that reveal a missing row. It never
supplies the all-clear.

---

## 7. Stage 5 — PLAN (not built)

Input: the extraction manifest, the invariants from stage 2, the discharged
obligations from stage 3, the surviving mutants from stage 4.
Output: `plan.json`. **No code is emitted at this stage.**

Prior art for the shape: Camunda's `api-test-generator` (open, in production,
700+ generated tests merged) splits `path-analyser` (plans, writes JSON only)
from `materializer` (the only stage that emits). Its `request-validation` arm
derives 24 negative-scenario kinds, each from one spec keyword — `required`,
`type`, `oneOf`, `uniqueItems`, `multipleOf`, `format`, `allOf`, enum, param.
Same method, different rule source: for Rust the rules are §3.2's type table
plus §3.3's MIR facts.

Their limit is instructive and is **not** ours: with only a spec, their oracle
is the HTTP status code. With a correct reference solution, the exact expected
value is available — which is why the protocol has Hyperray execute every
declared case against the reference and generate assertions itself.

For selection, the state of the art is Imandra's **Region Decomposition** —
partition the input space into regions of uniform behavior and take one case
per region, a semantic analysis rather than a syntactic one. That is the
enumerate-the-finite-space rule done properly, and it is the correct target
for `graph.max_depth` expansion.

Three jobs, in order of importance:

1. **Admissibility.** A row is admissible iff it derives from something the
   code *declares*: a type, an enum variant, a public item, a **named**
   constant, or a stage-2 invariant. A fact obtained only by executing the
   reference and recording what happened is **not** admissible.
   This is `evidence-rule.md` and the spec skill's `Free:` declaration, and
   it is the rule that would have prevented the session's one unfair test —
   `ReplayReader::seek` returns the *inner* reader's position, a mechanism
   detail the contract never owes.
2. **Deduplication.** Merge rows a single case observes. `Format` (3) ×
   `Result` (2) is not always 6 cases.
3. **Pruning.** Drop `Err` arms for operations with a proved-total
   `ensures`; stage 3 already discharged them.

Open: whether the Promise `count` in schema v2 can be *checked* against a
region decomposition rather than only declared by the author. That would turn
the counting pass from a human judgement into a compile error.

---

## 8. Stage 6 — EMIT (not built)

Reads `plan.json`. Writes **the test file and `instruction.md` from one pass**.

Prior art: **DScribe** (ICSE 2022) — one template carries both the test
skeleton and the documentation fragment; both are generated together. Its
opening claim is this project's thesis:

> Test suites and documentation capture similar information despite serving
> distinct purposes. Such redundancy introduces the risk that the artifacts
> inconsistently capture specifications.

and its payoff:

> By generating documentation from the same source as tests, DScribe... ensures
> that documentation is accurate since outdated documentation is flagged by
> failing tests.

DScribe is Java-only; the idea transfers, the code does not.

Why this stage is load-bearing: the two defects a reviewer caught on
noodles-296 were **prompt/test attribution gaps** — tests called
`cram::r#async::io::IndexedReader::new` and
`set_reference_sequence_repository` on the unified builder while
`instruction.md` named neither. Both are structurally impossible when a single
plan row emits the assertion and the sentence in the same pass.

Constraints inherited from the platform, not invented here: `instruction.md`
is GitHub-issue prose, no bullets, no headings, pure ASCII, ≤500 words. Word
budget is therefore a *scheduling* constraint on stage 5, not a stage-6
formatting problem — plan must know the budget when it selects rows.

---

## 9. Per-language surface

Only the two ends are language-specific. The manifest and everything from
stage 5 onward is shared.

The other three adapters now exist and were measured the same way this one
was: `cpp_adapter.md`, `go_adapter.md`, `python_adapter.md` (2026-09-02). This
table is superseded by them wherever they disagree — two cells below were
written here from reasoning and were **wrong**, corrected in place:

| stage | Rust | Python | C++ | Go |
|---|---|---|---|---|
| extract | Charon / rustdoc JSON / MIR | `ast` + `dis` + annotations | **libclang bindings** (~~`-ast-dump=json`~~) | `go/types` + `go/ssa` |
| bound | RustHorn → LoopInvGen | LoopInvGen (SyGuS is language-free) | same | same |
| prove | Kani → CBMC | CrossHair (Z3) / Nagini | **ESBMC + explicit flags** (~~CBMC~~) | **no BMC exists** — Gobra needs annotations |
| adequacy | cargo-mutants / IVC | mutmut / cosmic-ray | mutate + re-prove (mull unavailable) | go-mutesting (broken) |
| plan / emit | shared | shared | shared | shared |

Corrections, both measured:

- **C++ extract.** `clang -Xclang -ast-dump=json` produced **354 MB** for a
  90-line file (1.0 MB with STL excluded — 343× overhead), and
  `-ast-dump-filter` emits concatenated JSON documents that fail to parse. A
  libclang extractor did the same job in **0.19 s / 6,850 bytes** — 51,775×
  smaller. Use the bindings, not the dump.
- **C++ prove.** CBMC is not the C++ choice here; ESBMC is, and it must be
  invoked with explicit checks (see below).

Two findings from the ports that change rules in *this* document:

**Stage 2's `false` rule is not enough.** §4 states that a `false` invariant
is a translation bug. Both the Python and Go runs found worse cases:
LoopInvGen reports **PASS under `-v` on a vacuous `false` invariant**, and a
guard-shaped `post_fun` makes it **echo the guard back verbatim** — output that
looks like an answer and proves nothing. The adapter must reject a returned
invariant that is `false`, or syntactically equal to the guard, before using it.

**Sampling loses to enumeration, measured.** Go's native fuzzer missed an
`int32` overflow across **16,426,540 executions in 30 s**; 49 enumerated
boundary values from stage 1 found **18 counterexamples immediately**. Feeding
stage-1 domains into the fuzzer as seeds found it in 3.3 s. This is the
strongest available evidence for the project's core claim that a finite
enumerated domain beats random sampling.

Stage 2 remains fully language-independent: LoopInvGen consumes a SyGuS
invariant problem, and any front end that can emit `pre_fun` / `trans_fun` /
`post_fun` gets the same service. Only the *translation into* that form is
per-language, and that translation is stage 1's job.

**Scope limit on all three ports.** Every C++, Go and Python number was
measured on small files the author wrote (25–121 lines). **None ran against a
real patch.** Only this Rust document is backed by a production patch
(noodles-296, 14 files). The port numbers are not comparable until that
changes.

---

## 10. The compiler assumption

Kani proves at MIR level. The verifier runs a compiled binary. If `rustc`
miscompiled, a MIR-level proof might not describe what actually ran.

This is a real gap and it is not hypothetical: Csmith found **325 previously
unknown bugs** across 11 C compilers, and *every* compiler tested was found to
"silently generate wrong code when presented with valid input" (Yang et al.,
PLDI 2011).

Three reasons it is small here, in increasing order of strength.

**1. Hyperray compares; it does not ship.** Apple's proof must survive
compilation because the binary is the product. Hyperray's question is
relative: does the submission behave like the reference over the bounded
scope. Reference and submission cross the same `rustc`, same flags, same
commit, same container. A miscompilation applies to both sides and cancels.
The absolute-correctness burden is Apple's; it is not this project's.

**2. `finalarchitecture.md` §1.4 already catches the residue.** The reference
must pass the verifier — real compiler, real binary, real machine. A
proof-to-binary transfer failure surfaces there as a failing test, loudly. It
cannot become a silent pass.

**3. The assumption is removable, and the removal is automatic.** seL4 did not
write a verified compiler; it validated each compilation after the fact:

> Our approach is **post-hoc translation validation**. We have developed a
> binary verification toolchain which takes the machine code produced by the C
> compiler, and **automatically proves that it is a correct translation** of
> the corresponding C source code.

Unmodified gcc 4.5.1, full proof at `-O1`, **98% coverage at `-O2`**, SMT-driven
(SONOLAR, Z3, HOL4). Outcome: "the compiler and linker need not be trusted."

They evaluated the verified-compiler route and rejected it — CompCert cost
performance and still did not close the chain, because its Coq C semantics
could not be reconciled with their Isabelle ones. **Validating one compilation
beat trusting a proved compiler.**

That shape is per-run, not per-compiler, which is exactly Hyperray's unit of
work: one task, one patch, one build. Not required for v1. Recorded because
the escalation path exists and is mechanical.

For Rust specifically the same technique at the LLVM layer is **untested by
this project** — Alive2 does translation validation for LLVM optimizations, and
`rustc` lowers through LLVM, but no measurement was taken here.

---

## 11. Honest status

| stage | status |
|---|---|
| 1 EXTRACT | exists (Charon; fallbacks measured) |
| 2 BOUND | exists (LoopInvGen measured on both real loops); Rust→SyGuS translation still manual |
| 3 PROVE | exists, found a real defect; generic type-witness selection still manual |
| 4 ADEQUACY | tools exist; not wired to `spec.md` rows |
| 5 PLAN | **not built** |
| 6 EMIT | **not built** |

Proved on noodles-296: 6 `ReplayReader` functions, 1 contract, 1 real `async
fn`, 8 functions across 4 crates via autoharness. One real defect found. Two
production loops bounded without any unwind limit.

Not proved: the remaining 13 files; `detect` needs stubs before it will
terminate.

Untested: everything outside Rust.

---

## 12. Standing rules

- **Compiling is not verifying.** Report which one happened.
- **Every bound cites a constant in the patch.** An invented bound can hide a
  defect and produce a meaningless pass.
- **A `false` invariant is a translation bug**, never a result.
- **Search before declaring a limitation.** In one session, every one of
  eleven claimed limits — generics, async, harness authoring, contracts,
  dependency size, unbounded loops, invariant search space, Rust→maths
  translation, plan, emit, stub soundness — already had a documented answer,
  usually one paragraph further down the same page. The oldest was Karr 1976.
- **Measuring the reference and asserting what you observed produces unfair
  tests.** The reference answers more questions than the contract asks. The
  contract is the filter; that is `evidence-rule.md`, and it is why plan
  (§7.1) exists at all.
- **Kani emits a test only when it finds a defect.** A passing proof emits
  nothing — that is not a coverage signal.
