# Go adapter

Status: **DESIGN — 2026-09-02**

The Go instantiation of the Hyperray language-adapter protocol
(`lang-adpaters/PROTOCOL.md`). It states which external tool fills which stage,
what each one was measured to do, and what remains unbuilt.

Every measurement in this document was produced on 2026-09-02 on macOS 27.0
(darwin/arm64), Go 1.26.0, Docker 29.5.2, against purpose-built Go packages in
`/tmp/goadapter` shaped after the noodles-296 patch that `rust_adapter.md` was
measured on. Nothing here is cited from memory. Where a claim is untested, it
says so. Where a tool failed to run, the failure is reported verbatim rather
than replaced by a citation.

**The headline result, stated before the detail:** Go's stage 1 is the
*strongest* of any language in the portfolio — better than Rust's, because the
extraction API ships in the standard library and needs no nightly compiler, no
external IR project, and no unstable flags. Go's stage 3 is the *weakest*.
There is no Kani for Go. §5 says exactly what exists instead and what each
option costs.

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
   [1] EXTRACT      |  go/ast + go/types + x/tools/go/ssa   (BUILT, measured)
                    v
             extraction manifest  (PROTOCOL fmt 1 or graph fmt 2)
                    |
   [2] BOUND        |  SSA -> SyGuS -> LoopInvGen           (loop invariants)
                    v
             manifest + Invariants column filled
                    |
   [3] PROVE        |  NO BOUNDED MODEL CHECKER EXISTS.
                    |  Gobra (annotations, unsound by default on overflow)
                    |  + go test -fuzz + race detector
                    |  + exhaustive enumeration over stage-1 domains
                    v
             partial evidence; see §5 for what "verified" can mean
                    |
   [4] ADEQUACY     |  gremlins (+ equivalence check)
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

Stages 1, 2 and 4 are measured working below. Stage 3 is the honest gap.
Stages 5-6 do not exist for Go, exactly as they do not exist for Rust.

---

## 3. Stage 1 — EXTRACT

**This is Go's strongest stage, and it is stronger than Rust's.** Rust needs
Charon (a separate research project), a nightly compiler, and `-Z` flags.
Go ships the whole extraction surface in its own standard library and in one
`golang.org/x/tools` module that the language team maintains.

### 3.1 What was built and run

A real extractor, `goextract` (410 lines, `/tmp/goadapter/extract/main.go`),
using `golang.org/x/tools/go/packages` (v0.49.0) with
`NeedTypes | NeedTypesInfo | NeedSyntax | NeedDeps`. It loads a package with
full type information and walks every function body.

Subject: `/tmp/goadapter/target` — a 121-line package deliberately shaped like
the noodles-296 patch (named constants, an enum-like closed type, a
data-dependent loop, a bounded loop, an arithmetic obligation, a nil-map
write).

```sh
go build -o bin/goextract ./extract    # build 0.66s
bin/goextract ./target > manifest.json # run 0.66s, 13762 bytes
```

**Measured output, verbatim from stderr:**

```
SUMMARY functions=7 types=2 body_constants=28 branches=10 loops=2
        obligations=13 domains=10
  obligation index-out-of-range               3
  obligation integer-wraparound(int)          3
  obligation integer-wraparound(int32)        1
  obligation integer-wraparound(int64)        1
  obligation make-negative-len                2
  obligation nil-map-write                    1
  obligation slice-bounds                     2
```

### 3.2 The four things it recovered

**Signatures, fully typed** — 7 functions including methods with receivers:

```
reader.go:43:1  ScratchLen   func(dstLen int) int
reader.go:56:1  ReadAtLeast  func(r io.Reader, buf []byte, want int) (int, error)
reader.go:72:1  Detect       func(r io.Reader) (target.Format, error)
reader.go:108:1 SeekCurrent  func(offset int64) (int64, error)
reader.go:119:1 Widen        func(a int32, b int32) int32
```

**Closed variant sets** — the enum-equivalent. Go has no `enum`, but a named
integer type with `iota` constants is one, and `go/types` recovers it exactly:

```
Format  basic-alias  variants = [FormatUnknown=0, FormatGzip=1,
                                 FormatBgzf=2, FormatRaw=3]
```

**Constants inside function bodies** — rustdoc JSON's documented blind spot,
which forced the Rust adapter into a second MIR dump. Go has no such blind
spot: `go/types`'s `Info.Types` carries a constant-folded value for every
expression, and `Info.Uses` resolves every identifier to its `types.Const`.

Measured: **28 body constants — 15 literals, 13 named references.**

```
distinct literals : 0:int  1:int  2:int  3:int  31:byte  139:byte  4:byte
distinct named    : MagicLen  MaxDetectionPrefixLen  ScratchStep
                    FormatUnknown FormatGzip FormatBgzf FormatRaw
```

`31`, `139`, `4` are the gzip magic bytes `0x1f`, `0x8b`, `0x04` written as
hex in the source. Those three values are seed domain members that no
signature-level extraction could ever produce, and Hyperray gets them from a
single pass with no second tool.

**Branch points with exact arm lists** — the `switchInt` equivalent, 10 sites.
The tagless `switch` in `Detect` came back with its three arms enumerated:

```
reader.go:93:2  switch  <tagless>
  arms: ["plen >= 2 && prefix[0] == 0x1f && prefix[1] == 0x8b",
         "plen == 0", "default"]
```

**Loops, classified against a source constant:**

```
reader.go:58:2  ReadAtLeast  for  cond="dlen < want"
                statically_bounded=false
reader.go:75:2  Detect       for  cond="plen < MaxDetectionPrefixLen"
                statically_bounded=true  bound_source="constant 65536"
```

That `bound_source` is the `Evidence` cell of a spec row, and it satisfies the
standing rule that every bound cites a constant already in the patch.

### 3.3 SSA, and the cross-check that matters

A second tool, `gossa` (`/tmp/goadapter/gossa/main.go`), builds real SSA via
`golang.org/x/tools/go/ssa` with `ssautil.AllPackages`.

```
SSA TOTALS blocks=38 instructions=118 if-terminators=18 back-edges(loops)=2
  ssa.IndexAddr (bounds obligation)     x3
  ssa.MapUpdate (nil-map obligation)    x1
  ssa.Slice (slice-bounds obligation)   x3
```

Per function:

| function | blocks | instrs | ifs | back-edges |
|---|---|---|---|---|
| `Detect` | 17 | 51 | 11 | 1 |
| `ReadAtLeast` | 9 | 22 | 4 | 1 |
| `ScratchLen` | 5 | 10 | 2 | 0 |
| `*ReplayReader.SeekCurrent` | 1 | 14 | 0 | 0 |
| `*ReplayReader.Label` | 1 | 4 | 0 | 0 |
| `Widen` | 1 | 2 | 0 | 0 |

Back-edges are computed with `BasicBlock.Dominates`, not block-index
comparison. **A first implementation used `succ.Index <= b.Index` and reported
17 back-edges for a package with 2 loops** — an 8.5× overcount, because SSA
block numbering is not topological. The dominator test gives exactly 2, which
matches the 2 `for` statements the AST pass found. Recording the bug because
the wrong version looks plausible and would have produced a nonsense unwind
budget.

**Three independent passes agree on the obligation count.** The AST pass says
3 index + 2 slice; SSA says 3 `IndexAddr` + 3 `Slice`; and the Go compiler's
own bounds-check-elimination debug pass names 5 surviving runtime checks:

```sh
go build -a -gcflags="-d=ssa/check_bce/debug=1" ./...
./reader.go:62:23: Found IsSliceInBounds
./reader.go:83:31: Found IsSliceInBounds
./reader.go:94:26: Found IsInBounds
./reader.go:94:47: Found IsInBounds
./reader.go:95:32: Found IsInBounds
```

This is the direct analogue of rustc's `assert(... would overflow)` markers in
the MIR dump: **the Go compiler publishes which of its own safety obligations
it could not discharge.** The SSA/AST discrepancy is explained: SSA counts the
third `Slice` from `prefix = append(prefix, buf[:n]...)` where BCE proved one
of the two bounds. The agreement is the useful part — a check the compiler
eliminated is a check Hyperray need not enumerate.

Also available and measured: the compiler's interval-analysis `prove` pass,
which reports what it established about the arithmetic:

```sh
go build -a -gcflags="-d=ssa/prove/debug=2" ./...
./reader.go:48:16: x+d > w; x:v6 b2 delta:8192 w:65536 d:signed
./reader.go:94:26: Proved v143's arg 1 (v140) is constant 0
./reader.go:76:21: Disproved Leq64 (v42)
```

### 3.4 What does NOT work, measured

**`go doc -json` does not exist.** The task brief listed it; Go 1.26.0 rejects
it:

```
$ go doc -json ./
flag provided but not defined: -json
```

`go doc` has `-all`, `-short`, `-src`, `-u`, `-http`, `-C`, `-c`, `-cmd` — no
JSON output at any version on this machine. The rustdoc-JSON analogue for Go
is `go/types` itself, which is better: it is typed, it is not a text format,
and it covers bodies.

`go build -gcflags=-S` works (1124 lines of arm64 assembly for the package)
but is the wrong altitude — it is post-register-allocation machine code. It
did confirm one fact usefully: `runtime.mapassign_faststr` appears at
`reader.go:115`, the nil-map write site. Use SSA, not assembly.

### 3.5 What extraction feeds

| manifest field (PROTOCOL) | source | measured |
|---|---|---|
| `operations[].domains` | type table (§3.6) + `types.Const` variant sets | yes |
| domain `source expression` | `types.Signature.String()` | yes |
| `graph.seeds` (fmt 2) | typed value domains + body literals | yes |
| `graph.transitions` | exported funcs; ownership is pointer-vs-value | partial |
| exclusions | BCE-eliminated checks; `prove`-pass facts | yes |
| constants in row outcomes | `Info.Types[expr].Value` | yes |

### 3.6 Type → domain table

The `Parameters:` line of a spec row. Implemented in `domainFor()` and
exercised on the subject package; 10 domains emitted.

| type | finite domain values |
|---|---|
| `intN` / `uintN` | min, max, -1, 0, 1 (wraparound witnesses) |
| `bool` | false, true |
| `string` | `""`, `"a"`, invalid UTF-8 |
| `[]T` | **`nil`**, `[]T{}`, one, many |
| `map[K]V` | **`nil` (writes panic)**, empty, one entry |
| `*T` | `nil`, non-nil |
| `error` | `nil`, non-nil |
| named int + `iota` consts | one value per constant |
| `chan T` | nil, unbuffered, buffered, closed (**untested**) |

Two entries carry Go-specific hazards Rust does not have. `nil` and `[]T{}`
are **distinct** for a slice and must both appear. A `nil` map reads fine and
panics on write — it is the only type in Go whose zero value is a partial
value, and §5.3 shows it is a defect class the tooling actually catches.

**Blind spots, stated plainly:** interfaces are open (any type may implement
`io.Reader`, so `Detect`'s domain is unbounded until a witness type is
chosen — the same generic-witness problem `rust_adapter.md` §5.6 hit, and it
has the same answer: an interface→witness table the adapter owns);
type parameters (Go generics) were **not tested**; `unsafe` and cgo were
**not tested**.

---

## 4. Stage 2 — BOUND (loop invariants)

Language-independent, exactly as `rust_adapter.md` §4 predicted. LoopInvGen
consumes a SyGuS invariant problem and does not know or care that the loop
came from Go.

**Tool: LoopInvGen** (`SaswatPadhi/LoopInvGen`, MIT, Docker image
`padhi/loopinvgen`, pulled 2026-09-02, runs under `--platform linux/amd64`
emulation on arm64).

The transition relations were derived **from the SSA in §3.3**, not from the
source text. `ReadAtLeast`'s SSA gives the phi node, the guard, and the
update directly:

```
3: for.loop
   t2  = phi [0: 0:int, 7: t11] #dlen     -> state variable
   t3  = t2 < want                        -> loop guard
1: for.body
   t1  = t2 >= len(buf)                   -> early-exit condition
5: t5  = slice buf[t2:]                   -> read is bounded by blen - dlen
7: t11 = t2 + t7                          -> dlen' = dlen + n
```

### 4.1 Measured

| case | invariant found | time |
|---|---|---|
| `ReadAtLeast` (reader.go:58) | `0 <= dlen && dlen <= blen` | 1s |
| `Detect` (reader.go:75) | `0 <= plen && plen <= 65536` | 0s |
| coupled counters (Karr probe) | **`2i = s`** | 1s |
| contradictory relation (control) | **`false`** | 1s |

The `2i = s` result reproduces the Rust adapter's finding on Go-shaped input:
an *equality between variables*, outside the conjunctions-of-inequalities
class. Karr 1976.

`Detect`'s `0 <= plen <= 65536` is the useful one: the loop's unwind bound is
now a proved fact citing `MaxDetectionPrefixLen`, a constant stage 1 read out
of the source.

### 4.2 A vacuous postcondition produces a vacuous invariant

First attempt at both real loops used `post_fun` of the shape
`(or <loop guard> <property>)`. LoopInvGen returned:

```
Detect:      (or (< plen 65536) (and (>= plen 0) (<= plen 65536)))
bad-model:   (or (< dlen want) (<= dlen blen))
```

Both are `post_fun` echoed back verbatim — trivially inductive, and worth
nothing. Rewriting `post_fun` as the bare property, with no guard disjunct,
produced the real invariants in the table above.

**Rule, and it is new — `rust_adapter.md` did not have it:** a returned
invariant that is syntactically equal to `post_fun` is a **vacuous result**,
not a proof. Check for it mechanically before recording anything in the
`Invariants` column. It is the same class of error as the `false` rule, and it
is more dangerous because `false` is obviously wrong and this is not.

The `false` rule reproduces too: a deliberately contradictory transition
relation returned `(define-fun inv_fun ... false)`. A `false` invariant is a
translation bug, never a proof.

### 4.3 Where the transition relation comes from

Hand-written today, from the SSA. **No Go→CHC or Go→SMT frontend was found.**
Searched for a RustHorn equivalent; there is none. This is real work the
adapter must do, and §3.3's SSA is the right input for it: `ssa.Phi` gives the
state variables, `ssa.If` the guards, `ssa.BinOp` the updates, and
`BasicBlock.Dominates` the loop structure. Every ingredient is already
extracted; only the emitter is missing.

Output lands in the spec's `Invariants` column.

---

## 5. Stage 3 — PROVE (the weak stage — read this section carefully)

**There is no Kani for Go. There is no CBMC for Go. Go has no mature bounded
model checker, and the adapter must not pretend otherwise.**

Rust's stage 3 produces `T(C)=true` over a bounded scope by pushing MIR
through GOTO into a SAT/SMT formula. Nothing does that for Go. What follows is
everything that was actually installed and run, with what each one proved and
what it cannot.

### 5.1 The subject

`/tmp/goadapter/bugs` — five planted defects, one per class, plus a correct
control:

| # | defect | function |
|---|---|---|
| 1 | int32 multiplication wraparound | `Widen` |
| 2 | subtraction underflow into a length | `Unread` |
| 3 | index out of range (`prefix[3]` after `len>=2` check) | `Classify` |
| 4 | nil map write | `(*Counter).Add` |
| 5 | data race | `RacyCount` (separate package) |
| — | correct, for false-positive checking | `Clamp` |

### 5.2 What each tool found — the scoreboard

| tool | 1 overflow | 2 underflow | 3 index | 4 nil map | 5 race | control clean |
|---|---|---|---|---|---|---|
| `go vet` | no | no | no | no | n/a | yes |
| `staticcheck` (2026.1) | no | no | no | no | n/a | yes |
| `nilness` (x/tools) | no | no | no | no | n/a | yes |
| `go test -fuzz` 30s | **NO** | no | **yes 0.7s** | **yes 0.5s** | n/a | yes |
| `go test -fuzz` + seeded corpus | **yes 3.3s** | — | — | — | n/a | — |
| `testing/quick` 100k | **yes** | no | — | — | n/a | yes |
| exhaustive over §3.6 domains | **yes** | **yes** | **yes** | — | n/a | yes |
| Gobra (default) | **NO — verifies a false postcondition** | **yes** | **yes** | **yes** | **yes** | yes |
| Gobra `--overflow` | **yes** | yes | yes | yes | yes | yes |
| `go test -race` | n/a | n/a | n/a | n/a | **yes 8s** | yes |
| Gomela + SPIN | — | — | — | — | **tool broken** | — |

Three findings in that table are load-bearing. They follow.

### 5.3 Finding 1 — fuzzing missed the overflow that enumeration caught instantly

`FuzzWiden` ran **16,426,540 executions in 30 seconds** (≈550k/sec, 10
workers) and reported `new interesting: 0 (total: 1)`. It did not find the
int32 overflow.

The same defect, by exhaustive enumeration over the stage-1 domain
`{MinInt32, -1, 0, 1, MaxInt32, 65536, 46341}`:

```
exhaustive 49 pairs, 18 counterexamples
EXHAUSTIVE COUNTEREXAMPLE Widen(65536,65536)=0 exact=4294967296
EXHAUSTIVE COUNTEREXAMPLE Widen(2147483647,2147483647)=1 exact=4611686014132420609
```

**49 cases beat 16.4 million.** The reason is structural, not incidental: Go's
fuzzer is coverage-guided, `a*b` has exactly one basic block, so every input
produces identical coverage and the mutator has no gradient to follow. It is
performing uniform random search over 2⁶⁴ pairs for a target set that is a
vanishing fraction of it.

Confirmed by controlled experiment. Same function, same oracle, same 30s
budget, only the seed corpus changed — adding the four boundary values stage 1
derives from `int32`:

```
FuzzWidenSeeded: failure while testing seed corpus entry: seed#3
  Widen(65536,65536)=0 exact=4294967296          [3.3s, at seed time]
```

The fuzzer never had to mutate anything; the answer was in the corpus.

**This is the single most important operational fact in this document.**
Stage 1 already computes the boundary values (§3.6). Feeding them to `f.Add()`
converts Go's fuzzer from a random search into a directed one. An unseeded
`go test -fuzz` run is not evidence of absence for arithmetic defects, and a
green 30-second fuzz result must never be recorded as one.

`testing/quick` found it at 100k random cases because its `int32` generator
draws from the full range rather than mutating a corpus — worth keeping as a
cheap second opinion, but it is still sampling.

Where fuzzing *is* excellent: both panic-class defects, from the seed corpus,
in under a second each, with a full stack trace naming the line:

```
FuzzClassify: panic: runtime error: index out of range [3] with length 2
              bugs.go:20                                        [0.7s]
FuzzAdd:      panic: assignment to entry in nil map
              bugs.go:35                                        [0.5s]
```

`go test -fuzz` also writes the failing input to `testdata/fuzz/` as a
permanent regression seed — the analogue of Kani's concrete playback, and it
happens automatically.

### 5.4 Finding 2 — Gobra is unsound for integer overflow by default

Gobra (`ghcr.io/viperproject/gobra:latest`, rev `810de065`, built 2026-08-18,
ETH Zurich, Viper/Silicon backend, Z3) is a real deductive verifier for Go
based on separation logic. It works. It found three of the four sequential
defects and proved the correct functions correct.

But this file **verifies clean** in Gobra's default configuration:

```go
// @ ensures res == a * b
// @ decreases
func Widen(a int32, b int32) (res int32) {
	return a * b
}
```

The postcondition is **false**: at runtime `Widen(65536, 65536) == 0`, as §5.3
measured. Gobra accepted it.

Matrix, run four times on the same file, isolated:

| flags | result |
|---|---|
| (default) | **VERIFIED — unsound** |
| `--int32` | **VERIFIED — unsound** |
| `--overflow` | `Expression a * b might cause integer overflow` |
| `--overflow --int32` | `Expression a * b might cause integer overflow` |

`--help` states it outright: `--nooverflow  Do not check for integer overflow
(default)`. Gobra's own documentation confirms the semantics and adds a second
caveat — *"Overflow checking is still an experimental feature under
development. You may encounter bugs and observe unexpected results."*

**Mandatory rule: any Gobra invocation in this pipeline passes `--overflow`.
A Gobra run without it is not evidence about any arithmetic property.** Note
also that `--overflow` degrades error messages — under it, index-permission
failures at `bugs.go:25` were reported under the heading `Expression may cause
integer overflow` with the body `Permission to prefix[0] might not suffice`.
Read the body, not the heading.

### 5.5 Finding 3 — Gobra does prove real things, at a real cost

With `--overflow` and annotations, Gobra discharged genuine obligations.

**Positive results** (`/tmp/goadapter/gobra/annotated/bugs.go`, 15-18s per
run, three runs: 17.8s / 16.9s / 15.2s):

- `ScratchLen` — proved `0 <= res <= 8192` **and** `res + dstLen <= 65536`
  for every `dstLen` in `0..65536`. Those are the two spec rows the Rust
  adapter discharged by Kani contract in 0.04s. Gobra does it in ~15s with a
  precondition and two postconditions written by hand.
- `ReadAtLeastLoop` — the stage-2 invariant `0 <= dlen && dlen <= blen`,
  transcribed from LoopInvGen's output into a Gobra `invariant` clause,
  discharged the postcondition. **This is the stage-2→stage-3 handoff working
  end to end**, executed by hand here, and it is the strongest single result
  in this document: a synthesised invariant became a proof obligation another
  tool discharged.
- `ClassifyFixed` (the repaired `len>=4` version) verified while `Classify`
  failed — so the tool distinguishes the bug from the fix rather than
  rejecting both.

**Negative controls all failed correctly** — a verifier that accepts
everything proves nothing, so this was tested explicitly:

```
neg.go:6   UnreadNoPre   Postcondition might not hold. Assertion res >= 0
neg.go:16  LoopWrongInv  Postcondition might not hold. Assertion res <= blen
neg.go:46  Add           Assignment might fail. Permission to c.hits
```

`LoopWrongInv` is the sharp one: identical to the verified loop except the
invariant says `dlen <= want` instead of `dlen <= blen`. Gobra rejected it.
A wrong stage-2 invariant does not silently produce a passing stage-3 proof.

**Race freedom, proved by construction.** Separation logic makes this
structural rather than heuristic:

```go
// @ requires acc(x)
// @ ensures acc(x)
func inc(x *int) { *x = *x + 1 }

func Sequential() { y := new(int); inc(y); inc(y) }   // VERIFIED
func Concurrent() { y := new(int); go inc(y); go inc(y) }
// Error at racefree.go:28: Precondition of call might not hold.
//   go inc(y) might not satisfy the precondition of the callee.
```

`acc(y)` cannot be given away twice. This is a **proof over all schedules**,
which no amount of `-race` testing provides.

**The cost, measured.** On the annotated file: **58 lines of Go, 20 annotation
lines — a ratio of 0.34.** For every three lines of Go, one line of
specification. And it is not Go: the annotations are `// @` comments the
compiler ignores, in a separate language with `acc`, `forall`, `decreases`,
predicates and permission fractions.

Rust's stage 3 requires **zero** source changes — `kani autoharness` generates
harnesses internally. Go's requires a third of a file's volume in a
specification language, written by whoever runs the pipeline. That is the
whole difference between the two adapters in one number.

Three further limits, measured:
- Unannotated Go verifies almost nothing. The plain file failed at
  `bugs.go:8:5` with `Permission to prefix[0] might not suffice` — Gobra
  cannot even *read* a slice without a permission precondition.
- Syntax is unforgiving and errors are internal-looking:
  `// @ unfold true` produced
  `Translation of predicateAccess true failed: Expected invocation
  viper.gobra.frontend.ParseTreeTranslator.visitPredicateAccess`.
- `&y` on a local is rejected: `property error: got y that is not effective
  addressable`. `new(int)` works. An emitter must know this.

Gobra is not a toy — VerifiedSCION, a CCS 2025 paper on a verified internet
router, and Huawei Cloud reliability work (EuroSys 2026) all use it. It is
the right tool for a hand-verified component. It is a poor fit for a pipeline
that must run unattended on an arbitrary patch.

### 5.6 What could not be made to work, and exactly how it failed

**`gobmc` does not exist.** The name in the task brief has no tool behind it.
The paper it points to is Dilley & Lange, *Automated Verification of Concurrent
Go Programs via Bounded Model Checking* (Royal Holloway / Kent), and the tool
is called **Gomela**.

**Gomela: installed, ran, produced a broken model.** Two attempts:

1. Native (Go 1.26.0): `go install github.com/nicolasdilley/gomela@latest`
   succeeded after correcting the module path case (the repo declares
   `nicolasdilley/gomela`; `@latest` on the capitalised path fails with
   `version constraints conflict`). Running it **panicked**:
   `go/types.(*StdSizes).Sizeof ... nil pointer dereference`, from vendored
   `x/tools@v0.1.5`.
2. Docker, `golang:1.19-bullseye` + `spin` + `coreutils`, built clean. Gomela
   then ran and **generated a Promela model** — but SPIN 6.5.2 cannot parse it:

```
$ spin -a 'main++main5.pml'
spin: main++main5.pml:62, Error: incomplete structure ref 'ch_ch'
      saw 'operator: !'
```

The generated model declares `proctype send13(Chandef ch_ch; ...)` and then
uses `ch_ch!0` — sending on a struct rather than its `sync` field. It also
calls `run receiver(...)` where `receiver` is never defined. **This is Gomela's
own README hello-world example, verbatim**, which the README documents as
producing `Number of states: 29`. It does not, on this machine, at this
version, with SPIN 6.5.2.

Verdict: Gomela is 2023 research code that does not run against current
toolchains. It is not available to this pipeline. (The paper's claims are not
in dispute — the artifact is.)

**`go-mutesting`: installed, crashes.** See §6.

**Not attempted, and why:** `gollvm` → KLEE — gollvm is a source build of an
LLVM Go frontend, and the golang-nuts thread on exactly this idea has no
success report; the Go runtime's goroutine scheduler would have to be modelled
before KLEE saw anything meaningful. `GCatch` — targets concurrency
(blocking/non-blocking bugs), not the arithmetic and bounds classes this
pipeline needs; not installed. Both are **untested**.

### 5.7 What "PROVE" can honestly mean for Go

Given the above, the stage-3 verdict for Go is not one tool but a stack, in
descending order of strength:

1. **Exhaustive enumeration over the stage-1 finite domains.** For a *bounded*
   task — which is the only kind Hyperray accepts — this is not a weaker
   substitute for model checking. It **is** model checking, done concretely.
   If the domain is `{MinInt32, -1, 0, 1, MaxInt32}` and the code is executed
   on the full product, the result is exhaustive over that domain, and §5.3
   measured 49 cases dominating 16.4M random ones. The soundness argument is
   the domain's, not the executor's — and the domain comes from stage 1 with
   citations. This is where the adapter's engineering should go.
2. **Gobra with `--overflow`**, for properties that quantify over an unbounded
   domain (a loop invariant, a `forall` postcondition) where enumeration
   cannot close the gap. Costs 0.34 annotation lines per code line. Use it for
   the few functions where the spec row genuinely needs universal
   quantification, notably the stage-2 invariant handoff of §5.5.
3. **`go test -fuzz` with a stage-1-seeded corpus**, for panic-class defects
   (index, nil map, nil deref, slice bounds) where it is fast and precise, and
   where it writes a permanent regression seed for free.
4. **`go test -race`**, for concurrency. Precise and cheap — it named
   `conc.go:13` and both goroutines in 8s — but it is a *dynamic* detector: it
   sees only races on schedules that actually occurred. The unsynchronised
   counter returned `894` instead of `1000` on the run without `-race`, so the
   bug was even *observable* and the test still passed, because the test
   asserted nothing. `-race` proves nothing about schedules not taken; Gobra's
   `acc` does.

**None of these produces `VERIFIED` on its own**, and the boundary from
`finalarchitecture.md` applies here exactly as it does for Rust: only the four
Semantic-IR queries produce a verdict. What changes for Go is that the
`T(C)=true` obligation must be discharged by (1) with an explicit,
citation-backed domain argument, or by (2) with annotations — never by (3)
or (4).

---

## 6. Stage 4 — ADEQUACY (is the spec strong enough)

The false-positive direction of `finalarchitecture.md` §1.2. Mutate the code,
re-run the check; a surviving mutant means a missing spec row.

**Tool: gremlins** (`go-gremlins/gremlins` v0.6.0, installed via `go install`,
worked first try). `go-mutesting` (`zimmski/go-mutesting`) **installed but
crashes**: `panic: runtime error: invalid memory address ... 
go/types.(*StdSizes).Sizeof`, from vendored `x/tools@v0.0.0-20191018` — a 2019
snapshot against Go 1.26. Same root cause as Gomela's failure. Not usable.

### 6.1 Measured: weak suite vs. strong suite

Subject: `/tmp/goadapter/adequacy`, two functions (`ScratchLen`, `Clamp`),
11 mutants generated.

| suite | statement coverage | killed | lived | not covered | efficacy |
|---|---|---|---|---|---|
| happy-path only | 63.6% | 4 | 5 | 2 | **44.44%** |
| stage-1-derived boundaries | **100%** | 7 | 4 | 0 | **63.64%** |

The strong suite's cases were derived mechanically from §3.2's output — the
body constants `65536` and `8192` and the two branch conditions — taking each
boundary, one below, and one above. Coverage went 63.6% → 100% and mutant
coverage 81.8% → 100%.

**Operational hazard, measured:** the first gremlins run reported
`Timed out: 9, Killed: 0, Test efficacy: 0.00%`. The package's tests run in
under a second and gremlins' default timeout coefficient is too tight for
that. `--timeout-coefficient=20` fixed it completely. A 0% efficacy result on
a fast package is a configuration artifact, not a signal — check the timeout
column before believing any efficacy number.

### 6.2 100% coverage, 100% mutant coverage, and still 4 survivors

The strong suite left four `CONDITIONALS_BOUNDARY` mutants alive at
`adequacy.go:8:12`, `12:14`, `20:7`, `23:7`. Rather than assume the suite was
weak, each survivor was reconstructed by hand and enumerated exhaustively
(`/tmp/goadapter/equivcheck`):

```
ScratchLen 8:12  (d >= 65536 -> d > 65536)        71001 inputs -> EQUIVALENT MUTANT
ScratchLen 12:14 (d+n > 65536 -> d+n >= 65536)    71001 inputs -> EQUIVALENT MUTANT
Clamp      20:7  (v < lo -> v <= lo)              35301 triples -> EQUIVALENT MUTANT
Clamp      23:7  (v > hi -> v >= hi)              35301 triples -> EQUIVALENT MUTANT
```

**All four are equivalent mutants** — no input distinguishes them from the
original. `ScratchLen(65536)` returns 0 by either branch; `Clamp(v,lo,hi)`
returns `lo` whether the test is `<` or `<=` when `v == lo`. The suite's true
efficacy is **100%**, not the 63.64% gremlins reported.

This matters for the adapter's control flow: **a surviving mutant is a
question, not a verdict.** Feeding gremlins' 63.64% straight into a "missing
spec row" decision would have manufactured four spec rows for behaviours that
do not exist. The equivalence check is cheap — the same exhaustive
enumeration §5.7 already needs — and it must run before any survivor is
promoted to a missing row.

The published numbers say this is the normal case, not an edge case:
specifications in two case studies killed only 40% and 60% of mutants, and an
industrial 1250-line SCADE cruise-control killed 39% with every line marked
covered. Cheaper relative: **Inductive Validity Cores** — 24% overhead versus
2369% for mutation. Run IVC first, mutation on what it flags. **IVC has no Go
implementation and was not tested.**

Boundary, restated so the adapter never oversteps it:

> Coverage, PICT, mutation testing, fuzzing, property-based testing... may
> find useful witnesses. **None can produce `VERIFIED`.**

```text
EXISTS x,o: C(x,o) AND NOT R(x,o)                = UNSAT
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))       = UNSAT
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)     = UNSAT
T(C)                                             = true
```

---

## 7. Stage 5 — PLAN (not built)

Input: the extraction manifest, the invariants from stage 2, the discharged
obligations from stage 3, the surviving mutants from stage 4 **after the
equivalence filter of §6.2**.
Output: `plan.json`. **No code is emitted at this stage.**

Identical in shape to `rust_adapter.md` §7 — this stage is language-neutral by
construction, and nothing measured here changes it. Prior art for the split
(plan writes JSON, materializer is the only stage that emits): Camunda's
`api-test-generator`, whose `request-validation` arm derives 24
negative-scenario kinds each from one spec keyword. For selection, Imandra's
**Region Decomposition** — partition the input space into regions of uniform
behavior, one case per region — remains the correct target for
`graph.max_depth` expansion.

Three jobs, in order of importance:

1. **Admissibility.** A row is admissible iff it derives from something the
   code *declares*: a type, a `types.Const`, an exported item, a **named**
   constant, or a stage-2 invariant. A fact obtained only by executing the
   reference and recording what happened is **not** admissible. This is
   `evidence-rule.md`. Go's stage 1 makes this checkable rather than
   aspirational: every body constant in §3.2 arrives with a `types.Const`
   name or a source position, so an emitted row can carry its provenance
   mechanically.
2. **Deduplication.** Merge rows a single case observes. `Format` (4 variants)
   × `error` (2) is not always 8 cases.
3. **Pruning.** Drop rows for obligations stage 3 already discharged, and drop
   arms the compiler's BCE pass proved unreachable (§3.3) — Go gives this one
   for free, which Rust does not.

One Go-specific addition to the shared design: the plan must record **which
stage-3 mechanism** discharged each row (enumeration / Gobra / seeded fuzz /
race), because §5.7 ranks them and only the first two can support `T(C)=true`.
The Rust plan does not need this field; every row there says "Kani".

---

## 8. Stage 6 — EMIT (not built)

Reads `plan.json`. Writes **the test file and `instruction.md` from one pass**.

Prior art: **DScribe** (ICSE 2022) — one template carries both the test
skeleton and the documentation fragment, generated together:

> Test suites and documentation capture similar information despite serving
> distinct purposes. Such redundancy introduces the risk that the artifacts
> inconsistently capture specifications.

DScribe is Java-only; the idea transfers, the code does not.

Go's emit target is a `_test.go` file, and three details are Go-specific and
were exercised while measuring §5:

- **Emit `f.Add()` seeds, not just fuzz targets.** §5.3 is the whole argument:
  the same target with and without stage-1 boundary seeds is the difference
  between finding the defect in 3.3s and missing it in 16.4M executions. The
  seeds are already in the manifest.
- **Table-driven tests are the idiom**, and they are also the natural shape
  for an enumerated finite domain — one `[]struct{...}` literal per operation,
  one row per domain product element. The §6.1 strong suite is exactly this
  and it is what took mutant coverage to 100%.
- **A Gobra-annotated variant is a second artifact, not the same file.**
  Annotations are `// @` comments; they do not affect `go build`, so they can
  ship in the same file — but §5.5's 0.34 ratio means the plan must decide per
  row whether a Gobra annotation is worth authoring, and record that decision.

Constraints inherited from the platform, not invented here: `instruction.md`
is GitHub-issue prose, no bullets, no headings, pure ASCII, ≤500 words. Word
budget is a *scheduling* constraint on stage 5, not a stage-6 formatting
problem.

---

## 9. Per-language surface

Updated with what this session measured. Changes from `rust_adapter.md` §9 are
in **bold**.

| stage | Rust | Go |
|---|---|---|
| extract | Charon / rustdoc JSON / MIR (nightly, `-Z`) | **`go/types` + `go/ast` + `x/tools/go/ssa` — measured, stdlib, stable** |
| bound | RustHorn → LoopInvGen | **SSA → hand-written SyGuS → LoopInvGen — measured; no Go→CHC frontend exists** |
| prove | Kani → CBMC, zero source changes | **No BMC. Gobra (0.34 annotation ratio, `--overflow` mandatory) + seeded fuzz + enumeration** |
| adequacy | cargo-mutants / IVC | **gremlins — measured; go-mutesting crashes; no Go IVC** |
| plan / emit | shared | shared |

The two ends remain language-specific; the manifest and everything from stage
5 onward is shared. Stage 2 is confirmed language-independent by measurement:
LoopInvGen solved Go-derived problems with no Go-specific anything.

**The trade is now quantified.** Go wins stage 1 decisively — no nightly, no
external IR project, constants-in-bodies for free, and the compiler publishing
its own undischarged obligations. Go loses stage 3 just as decisively — Rust
gets a bounded proof with zero source changes; Go gets a bounded proof only by
enumeration over a domain it must justify, or by writing a third of a file in
a separate specification language.

Python's prove-stage tooling remains **unverified**. Do not copy this document
into `python-adapter.md` without running the tools; every number in this file
was measured, and the ports must hold the same standard.

---

## 10. Honest status

### 10.1 Per-stage

| stage | status | measured this session | NOT measured |
|---|---|---|---|
| 1 EXTRACT | **exists, built, run** | 410-line extractor: 7 fns, 2 types, 28 body constants, 10 branches, 2 loops, 13 obligations, 10 domains, 0.66s. SSA: 38 blocks, 118 instrs, 2 back-edges. BCE + prove passes cross-check to 5 surviving checks. | generics/type params; interfaces as domains; `unsafe`; cgo; multi-package patches; anything over 121 lines |
| 2 BOUND | **exists, measured** | 4 LoopInvGen runs, all ≤1s: two real Go loops, Karr equality probe, `false` control. Vacuity failure mode found and ruled. | Go→SyGuS emitter (hand-written today); arrays/ALIA logic; nonlinear loops; nested loops |
| 3 PROVE | **weak — no BMC exists** | Gobra on 6 files incl. 3 negative controls + race-freedom proof; overflow-flag matrix ×4; 5 fuzz targets (16.4M execs); seeded-corpus experiment; `testing/quick`; `-race`; exhaustive enumeration; `go vet`/`staticcheck`/`nilness` all silent | Gobra on anything realistic (>58 lines); Gobra + generics/interfaces/channels; `gollvm`→KLEE; GCatch; goroutine-count scaling |
| 4 ADEQUACY | **tools exist, one works** | gremlins: 11 mutants, weak 44.44% vs strong 63.64%, timeout artifact identified; all 4 survivors proved equivalent by 71001+35301 enumerated inputs | wiring survivors to `spec.md` rows; IVC (no Go implementation); mutation-vs-proof (needs a prove stage to re-run) |
| 5 PLAN | **not built** | — | everything |
| 6 EMIT | **not built** | — | everything |

### 10.2 Tools that failed, verbatim

| tool | attempt | failure |
|---|---|---|
| `go doc -json` | Go 1.26.0 | `flag provided but not defined: -json` — the flag does not exist |
| `gobmc` | web search | **no such tool.** The paper's artifact is Gomela |
| Gomela | `go install`, Go 1.26 | `panic: nil pointer dereference` in `go/types.(*StdSizes).Sizeof` via vendored `x/tools@v0.1.5` |
| Gomela | Docker, golang:1.19 + SPIN 6.5.2 | built and generated a model; **SPIN cannot parse it**: `incomplete structure ref 'ch_ch'`. Undefined `proctype receiver`. Failed on its own README example |
| `go-mutesting` | `go install`, Go 1.26 | `panic: nil pointer dereference` in `go/types.(*StdSizes).Sizeof` via vendored `x/tools@2019-10-18` |
| `gollvm` → KLEE | not attempted | source build of an LLVM Go frontend + Go runtime modelling; no success report found |
| GCatch | not attempted | targets concurrency classes outside this pipeline's needs |

### 10.3 The three results that should change a decision

1. **Seeded fuzzing beats unseeded fuzzing by orders of magnitude, and
   enumeration beats both.** 49 enumerated cases found an overflow that
   16,426,540 fuzz executions missed; adding four stage-1 boundary values to
   the corpus found it in 3.3s. Stage 1 already computes those values.
2. **Gobra's default configuration verifies a false postcondition about
   integer arithmetic.** `--overflow` is not optional; without it a Gobra pass
   is not evidence.
3. **All four surviving mutants were equivalent mutants.** Mutation efficacy
   is an upper bound on nothing until survivors are checked, and the check is
   the same enumeration stage 3 already needs.

### 10.4 Scope

Everything above was measured on packages of 25-121 lines that this session
wrote. **Nothing was measured on a real patch.** The Rust adapter's numbers
came from noodles-296, a 14-file production patch; these did not, and the two
documents are not comparable on that axis. Scaling Gobra, gremlins, and the
extractor to a real multi-package patch is untested and is the next
measurement to take.

---

## 11. Standing rules

Inherited from `rust_adapter.md` §11, plus four Go-specific rules this session
earned.

- **Compiling is not verifying.** Report which one happened. For Go, add: a
  green `go test -fuzz` run is not verifying either.
- **Every bound cites a constant in the patch.** `Detect`'s 65536 came from
  `MaxDetectionPrefixLen` via `types.Const`, not from an author's judgement.
- **A `false` invariant is a translation bug**, never a result.
- **NEW — an invariant syntactically equal to `post_fun` is vacuous.** Both
  real Go loops returned one on the first attempt. Check mechanically; it is
  more dangerous than `false` because it looks like an answer.
- **NEW — never run Gobra without `--overflow`.** Measured: the default
  verifies `ensures res == a*b` for `int32`, which is false at runtime.
- **NEW — never record an unseeded fuzz pass as evidence of absence.**
  16.4M executions, 30s, zero findings, defect present. Seed from the stage-1
  domain or enumerate instead.
- **NEW — a surviving mutant is a question, not a missing row.** Four of four
  survivors here were equivalent mutants. Enumerate before promoting.
- **Search before declaring a limitation.** This session's inverse also holds:
  **search before declaring a capability.** `gobmc` and `go doc -json` were
  both in the brief and neither exists; Gomela's published artifact does not
  run on its own example. Cite the run, not the paper.
- **Measuring the reference and asserting what you observed produces unfair
  tests.** The reference answers more questions than the contract asks.
- **Go's compiler tells you what it could not prove.** `-d=ssa/check_bce/debug=1`
  and `-d=ssa/prove/debug=2` are free stage-1 inputs with no analogue cost.
  Use them; a check the compiler eliminated is a row Hyperray need not write.
