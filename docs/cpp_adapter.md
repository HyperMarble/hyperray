# C++ adapter

Status: **DESIGN — 2026-09-02**

The C++ instantiation of the Hyperray language-adapter protocol
(`lang-adpaters/PROTOCOL.md`). It states which external tool fills which stage,
what each one was measured to do, and what remains unbuilt.

Every measurement in this document was produced on 2026-09-02 on macOS 27.0
(arm64) with ESBMC 8.4.0, CBMC 6.11.0, Apple clang 21.0.0, libclang 18.1.1 and
Docker image `padhi/loopinvgen`. Nothing here is cited from memory. Where a
claim is untested, it says so.

The subject is `replay_reader.cpp` — a C++ transliteration of the noodles-296
`ReplayReader` patch that `rust_adapter.md` measured, carrying the same defect
(`prefix.size() - position` on unsigned types) so the two documents can be read
against each other.

---

## 1. What the adapter is for

Unchanged from `rust_adapter.md` §1. Hyperray proves four things about one
fixed bounded task (`docs/specs/finalarchitecture.md` §1):

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
task-authored tests.

**What is different about C++.** Rust's adapter could assume the language was
the tractable part. C++ cannot. The single largest finding in this document is
that no bounded model checker on this machine verifies the *real* C++ standard
library; ESBMC verifies a **substitute** library it ships itself. That is a
soundness-relevant fact that belongs in the spec, not in a footnote — §5.6.

---

## 2. Stage map

```
        solution.patch + base checkout
                    |
   [1] EXTRACT      |  libclang Python bindings  (NOT clang -ast-dump=json)
                    v
             extraction manifest  (PROTOCOL fmt 1 or graph fmt 2)
                    |
   [2] BOUND        |  LoopInvGen (SyGuS)  -> __ESBMC_loop_invariant
                    v
             manifest + Invariants column filled
                    |
   [3] PROVE        |  ESBMC 8.4.0 (k-induction, contracts, incremental BMC)
                    v
             T(C)=true, safety obligations discharged
                    |
   [4] ADEQUACY     |  mutate source + re-run PROOF   (harness built here)
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

Stages 1–4 are measured below. Stages 5–6 do not exist for C++ either, and are
the adapter's actual engineering work. Unlike Rust, stage 4 for C++ has **no
off-the-shelf tool that installed** (§7.1) — the harness in this document is
70 lines and was written and run here.

---

## 3. Stage 1 — EXTRACT

### 3.1 The obvious tool is the wrong one

The task brief lists `clang -Xclang -ast-dump=json` first. It was measured
first, and it should not be the primary path.

```sh
clang++ -std=c++17 -Xclang -ast-dump=json -fsyntax-only replay_reader.cpp
```

| subject | wall time | JSON size |
|---|---|---|
| `replay_reader.cpp` (90 lines, includes `<vector>`) | 1.46 s | **354,658,350 bytes (354 MB)** |
| same file with all STL includes removed | 0.05 s | 1,034,648 bytes (1.0 MB) |
| `-ast-dump-filter=seek_current` (one method) | 0.19 s | 19,316 bytes |

354 MB of JSON from a 90-line file. The dump is dominated by libc++ template
instantiations; the ratio is **343×** between the STL and no-STL versions of
the same program. Parsing it costs a JSON reader that holds the whole document,
and `TranslationUnitDecl` reported 1,329 top-level `inner` nodes of which 8
were the subject's own.

`-ast-dump-filter` fixes the size but breaks the format:

- filter matching **one** declaration → one valid JSON document (verified with
  `json.load`).
- filter matching **two** declarations (`-ast-dump-filter=read`) →
  `json.JSONDecodeError: Extra data: line 224 column 1`. Clang concatenates
  one JSON document per matched declaration with no enclosing array and no
  separator. Measured, not assumed.

So the JSON path requires either 354 MB per file or a filter narrow enough to
match exactly one declaration, plus a stream splitter for when it does not.
Both are avoidable.

### 3.2 Primary: libclang Python bindings

`/tmp/cppadapter/extract.py`, 130 lines, is the measured extractor.

```sh
uv pip install libclang          # libclang==18.1.1
python extract.py replay_reader.cpp -isysroot $SDK -I$SDK/usr/include/c++/v1 \
    -I/Library/Developer/CommandLineTools/usr/lib/clang/21/include \
    -I$SDK/usr/include
```

| metric | value |
|---|---|
| wall time | **0.19 s** |
| manifest size | **6,850 bytes** |
| ratio vs. `-ast-dump=json` | 51,775× smaller, 7.7× faster |

It walks the cursor tree in-process and emits only what the protocol needs, so
the STL never reaches the output. **Set the `libclang.dylib` path explicitly**
(`ci.Config.set_library_file`); on this machine it is
`/Library/Developer/CommandLineTools/usr/lib/libclang.dylib`.

Measured output on the subject:

```
globals: MAX_DETECTION_PREFIX_LEN = 65536 ; SCRATCH_STEP = 8192
enum Format      -> Unknown, Bam, Cram, Vcf
enum SeekWhence  -> Start, End, Current
records: ReplayReader (2 fields; ctor, seek_current, at)

read_at_least  L21 size_t  params=[(want,size_t),(cap,size_t)]
    consts=['0'] loops=1 branches=1 subscripts=0  sub-ops=[('-',26,'size_t')×2]
detect_scan    L34 size_t  params=[(n,size_t)]
    consts=['0','0','1','2'] loops=1 branches=0
scratch_len    L44 size_t  params=[(dst_len,size_t)]
    consts=['0'] loops=0 branches=2 (if + ternary)  sub-ops=[('-',48,'size_t')]
seek_current   L61 int64_t params=[(offset,int64_t)]
    sub-ops=[('-',62,'unsigned long'), ('-',63,'int64_t')]   <-- the defect
at             L66 uint8_t params=[(idx,size_t)]  subscripts=1
detect_format  L71 Format  params=[(buf,const std::vector<uint8_t>&)]
    consts=['4','0','1','2','0','1','2','3'] branches=3 subscripts=7
clamp_add      L85 T       template_params=['T'] params=[(a,T),(b,T),(hi,T)]
```

Both constants that bound the loops, all seven enum variants, every branch
point with its arm count, and — critically — the two subtraction sites on line
62–63 that are the defect. This is the C++ equivalent of what
`rust_adapter.md` §3.2 got from the MIR dump: **the arithmetic obligations are
predictable from stage 1, before stage 3 runs.**

### 3.3 Three blind spots found by running it

Each of these was a wrong result first, then fixed. They are recorded because
any reimplementation will hit them.

1. **`v[i]` on a class type is not `ARRAY_SUBSCRIPT_EXPR`.** The Python binding
   (18.1.1) has **no `CursorKind.CXX_OPERATOR_CALL_EXPR`** — confirmed by
   `dir(CursorKind)`, which lists only `BINARY_OPERATOR`, `CALL_EXPR`,
   `COMPOUND_ASSIGNMENT_OPERATOR`, `CONDITIONAL_OPERATOR`, `UNARY_OPERATOR`.
   `std::vector::operator[]` arrives as a `CALL_EXPR` whose `spelling` is
   `operator[]`. Before the fix, `detect_format` reported 0 subscripts; after,
   7. A bounds-check obligation counter that misses `operator[]` misses every
   STL index in the program.
2. **`get_arguments()` returns nothing for a `FUNCTION_TEMPLATE`.**
   `clamp_add` reported `params=[]`. Its `PARM_DECL` and
   `TEMPLATE_TYPE_PARAMETER` children must be walked directly.
3. **The STL leaks into function bodies.** Walking a body that calls
   `v.size()` descends into libc++ headers and counts their literals and
   branches as the subject's. Every body-walker node needs
   `n.location.file.name == SRC`. The translation-unit cursor itself has
   `location.file is None` and must be exempted, or the walk returns nothing —
   that failure mode produced a 96-byte manifest.

Also measured: without `-isysroot`, libclang reports `'cstdint' file not
found` and silently returns a partial tree. Compile flags are **mandatory**
input to stage 1, not optional. Get them from `compile_commands.json` in a
real project.

### 3.4 What extraction feeds

| manifest field (PROTOCOL) | source |
|---|---|
| `operations[].domains` | type table (3.5) + `EnumConstantDecl` values |
| domain `source expression` | cursor `type.spelling` / `result_type` |
| `graph.seeds` (fmt 2) | typed value domains from the type table |
| `graph.transitions` | public methods + `CXXMethodDecl` const-ness |
| exclusions | `switch` arms with no matching `CASE_STMT` |
| constants in row outcomes | `INTEGER_LITERAL` tokens inside bodies |
| **unwind bounds** | namespace-scope `VAR_DECL` constants (§5.5) |

### 3.5 Type → domain table

This is the `Parameters:` line of a spec row.

| type | finite domain values |
|---|---|
| `intN_t` / `uintN_t` / `size_t` | min, max, one past each (overflow witness) |
| `bool` | true, false |
| `enum class E` | one value per `EnumConstantDecl` |
| `T*` | `nullptr`, valid, dangling |
| `std::vector<T>` | empty, one, many, `size()`, `size()+1` (OOB witness) |
| `std::string` | empty, one char, embedded NUL |
| `std::optional<T>` | `nullopt`, engaged |
| `T&` | always valid (no null witness) |
| template `T` | **not a domain** — needs a witness type, §5.9 |

Unlike Rust, C++ has no `Result`/`Option` convention to mine, and no ownership
information in the type. The `T*` row is where most C++ obligations come from,
and stage 3 checks it by default (§5.2).

---

## 4. Stage 2 — BOUND (loop invariants)

Same prior art as `rust_adapter.md` §4 — Karr 1976, Cousot & Halbwachs 1978,
Rodríguez-Carbonell & Kapur 2004, Monniaux (JACM 2025). Not repeated.

**LoopInvGen is language-independent and this is where that pays off.** It
consumes a SyGuS invariant problem (`synth-inv`, `pre_fun`, `trans_fun`,
`post_fun`); nothing in it knows what language the loop came from. The Rust
adapter's stage 2 transfers to C++ **unchanged**. Only the translation into
SyGuS is per-language, and that is stage 1's job.

### 4.1 Measured — both loops in the subject

Docker image `padhi/loopinvgen` (3.64 GB), run under emulation
(`linux/amd64` image on `arm64/v8` host — Docker warns; it works).

```sh
docker run --rm -v /tmp/cppadapter/sygus:/work padhi/loopinvgen \
  /home/opam/LoopInvGen/loopinvgen.sh /work/loop1_read_at_least.sl
```

| loop | site | invariant found | time |
|---|---|---|---|
| `read_at_least` (bounded buffer fill) | replay_reader.cpp:21 | `(and (>= len 0) (<= len cap))` | **1.14 s** |
| `detect_scan` (two coupled counters) | replay_reader.cpp:34 | `(and (or (not (>= i n)) (= seen (* 2 n))) (= (* 2 i) seen))` | **1.41 s** |

The second result is the important one. `(= (* 2 i) seen)` is an **equality
between two program variables** — Karr's 1976 class, outside conjunctions of
linear inequalities. §5.7 shows ESBMC's own k-induction **cannot** find it and
gives up; supplied with it, the same proof closes in 0.16 s. Stage 2 is not a
convenience for C++, it is load-bearing.

The `trans_fun` for loop 1 encodes the `if (len + got > cap) got = cap - len;`
branch as an `ite`, which is how a two-armed body becomes one transition
relation:

```lisp
(ite (> (+ len 8192) cap)
     (= len! cap)                ; got = cap - len  =>  len' = cap
     (= len! (+ len 8192)))
```

Both `8192` and `65536` are the constants stage 1 extracted from
`SCRATCH_STEP` and `MAX_DETECTION_PREFIX_LEN`. Nothing was invented.

### 4.2 An empty model reports `false`

Carried over from `rust_adapter.md` §4 and re-affirmed: a `false` invariant is
a **translation bug**, never a proof. Re-derive the transition relation.

### 4.3 Where the transition relation comes from — measured negative results

Hand-written today, as in the Rust adapter. C++ has fewer automatic options
than Rust does, and both candidates were tested:

**Frama-C / WP → requires Frama-Clang, and Frama-Clang is not viable.**
Not installed here; assessed from primary sources rather than run, and marked
as such. The official plugin page carries the maintainers' own caveat:

> Frama-Clang is currently in an early stage of development. It is known to be
> incomplete and comes without any bug-freeness guarantees.

The plug-in manual (v0.0.19) lists as *not implemented*: "uses of templates
are not robust", "implementation of the standard library is very rudimentary",
and states "main target of Frama-Clang is C++11". For a C++17 subject using
`std::vector`, this is not a path. **Untested by execution; rejected on
documentation.**

**CBMC's `goto-synthesizer` → ran, did not terminate.** CBMC ships an
automatic loop-invariant synthesizer, which would remove stage 2's manual step
entirely. Measured on the *simplest possible* input — a C (not C++) loop
`while (i < n) { i++; s += 2; }` with `n <= 1000`, compiled with `goto-cc`:

```sh
goto-cc -o gs.goto gs_loop.c
goto-synthesizer gs.goto gs_out.goto
```

**Killed at 692 s (11.5 minutes) with no output.** LoopInvGen answered the same
loop shape in 1.41 s. This is a single data point on one machine and the tool
may need flags I did not find, but the comparison is stark enough to set the
default: **LoopInvGen is the stage-2 tool; goto-synthesizer is not.**

So for C++ the SyGuS translation stays manual for now, and that is the honest
status. The `RustHorn` equivalent — a sound automatic C++→CHC front end —
is the missing piece, and unlike Rust there is no ownership discipline to
exploit in eliminating the heap.

### 4.4 The handoff to stage 3 works

This is the part `rust_adapter.md` could not show, because Kani has no
invariant-annotation input. ESBMC does, and the chain was measured end to end.
See §5.7.

---

## 5. Stage 3 — PROVE (ESBMC)

```sh
esbmc <file.cpp> --overflow-check --unsigned-overflow-check \
      [--unwind N | --k-induction | --incremental-bmc] --timeout 90s
```

ESBMC 8.4.0 was already installed. CBMC 6.11.0 was installed during this
session via `brew install cbmc` and is **not usable for C++** (§5.10).

### 5.1 The default invocation proves nothing, and says SUCCESSFUL

This is the single most dangerous measured behaviour in the whole pipeline.

```
$ esbmc b1_sub_overflow.cpp
Generated 0 VCC(s), 0 remaining after simplification (7 assignments)
VERIFICATION SUCCESSFUL
```

The program contains a guaranteed unsigned underflow. Slicing removed 156 of
163 assignments, **zero verification conditions were generated**, and the tool
printed `VERIFICATION SUCCESSFUL` in 0.63 s.

**Rule: a run that reports `Generated 0 VCC(s)` is not a proof.** The adapter
must parse the VCC count out of ESBMC's output and treat zero as an error, not
as a pass. This is the C++ counterpart of `rust_adapter.md`'s "compiling is not
verifying", and it is worse, because the tool actively reports success.

### 5.2 Which checks are on by default

Measured by running each bug with and without flags.

| bug | default flags | verdict | time |
|---|---|---|---|
| b2: `buf[i]`, `i <= 8` on `int buf[8]` | none needed | **FAILED** — `array bounds violated: array 'buf' upper bound` | 0.14 s |
| b3: null deref via `c > 0 ? new int(5) : nullptr` | none needed | **FAILED** — `dereference failure: NULL pointer`, CWE-476 | 0.15 s |
| b1: unsigned/signed subtraction | none | **SUCCESSFUL (false pass)** | 0.63 s |
| b1 | `--overflow-check` | **FAILED** — signed overflow, line 9 | 0.26 s |
| b1 | `--unsigned-overflow-check` | **FAILED** — unsigned overflow, line 8 | 0.26 s |

Bounds and pointer checks are **on by default** — which is why the opt-outs are
spelled `--no-bounds-check` and `--no-pointer-check`. Overflow is **off by
default and split in two**:

- `--overflow-check` catches the **signed** `offset - (int64_t)unread` on line 9.
- `--unsigned-overflow-check` catches the **unsigned** `prefix_len - position`
  on line 8, which is the *earlier* and more fundamental of the two.

Running only `--overflow-check` reports line 9 and never mentions line 8.
**The adapter must pass both.** (`--bounds-check` is not a flag; passing it
aborts with `unrecognised option '--bounds-check'` from Boost — measured.)

### 5.3 The defect, with counterexample

```
$ esbmc b1_sub_overflow.cpp --overflow-check
[Counterexample]
State 4  line 12  prefix_len = 0
State 5  line 13  position   = 1
State 6  line 14  offset     = 9223372036854775807
State 9  line 8   unread     = 0xFFFFFFFFFFFFFFFF
Violated property:
  file b1_sub_overflow.cpp line 9 column 5 function seek_current
  arithmetic overflow on sub
  CWE: CWE-190, CWE-191
  !overflow("-", offset::2, (signed long int)unread)
VERIFICATION FAILED
```

Concrete values, the exact SMT obligation, and CWE identifiers. The CWE tags
are free structured metadata for the spec row's `Evidence` cell — Rust's Kani
does not emit them.

The same defect on the realistic subject (`ReplayReader` with a real
`std::vector` member, harness 2): **703 VCCs, 166 after simplification,
FAILED at line 29 in 0.34 s**, same CWE pair.

And the control: `b5_clean.cpp`, the corrected `seek_current`, verifies
**SUCCESSFUL with 9 VCCs in 0.17 s** under both overflow flags. The tool is not
simply reporting failure on everything.

### 5.4 Unbounded loops: three behaviours, one of them unsound

Subject: `while (i < n) { i++; s += 2; }` with `n` nondeterministic, asserting
`s == 2*i` — a true statement requiring induction.

| invocation | result | time |
|---|---|---|
| default (no bound) | **ERROR: Timed out** after unwinding to iteration **4,526,073** | 95 s |
| `--unwind 10` | **FAILED — `unwinding assertion loop 3`** | 1.31 s |
| `--unwind 10 --no-unwinding-assertions` | **SUCCESSFUL** | 0.97 s |
| `--incremental-bmc` | `VERIFICATION UNKNOWN` — "forward condition unable to prove" | 2.77 s |
| `--k-induction` | `VERIFICATION UNKNOWN` — base case, forward condition **and inductive step at k=50** all fail | 5.14 s |

The third row is the trap. `--no-unwinding-assertions` turns a correct
"I could not reach a bound" into a clean `VERIFICATION SUCCESSFUL` that
covers only the first 10 iterations. **It is an unsound flag and the adapter
must never emit it.** The unwinding assertion in row 2 is the mechanism that
makes a too-small bound *visible*; suppressing it is exactly the false negative
of `finalarchitecture.md` §1.3.

Note also that `--k-induction` failing is not a bug — the property genuinely is
not k-inductive for any k the tool tried. That is what stage 2 is for.

### 5.5 The bound decides the answer — and it is derivable

Harness 1 on the realistic subject (`read_at_least`, bounded by the two
extracted constants):

| bound | verdict | time |
|---|---|---|
| `--unwind 8` | **FAILED — unwinding assertion loop 22** | 0.33 s |
| `--unwind 9` | **SUCCESSFUL** | 0.36 s |
| `--unwind 12` | **SUCCESSFUL** | 0.41 s |
| `--k-induction` (no bound given) | **SUCCESSFUL** — *"Solution found by the forward condition; all states are reachable (k = 9)"* | 0.61 s |

`9 = 65536/8192 + 1 = MAX_DETECTION_PREFIX_LEN / SCRATCH_STEP + 1`, and both
constants came out of stage 1 §3.2. **k-induction independently discovered the
same k=9.** That agreement is the strongest available evidence that the bound
is a property of the code and not of the author.

**Rule, inherited from `rust_adapter.md` §5.4 and re-confirmed here**: every
bound cites a constant already present in the patch. Where k-induction
terminates, prefer it — it needs no bound at all and reports the k it found,
which can be checked against the arithmetic.

### 5.6 C++ specific: ESBMC does not verify the real standard library

This is the finding that most changes what a C++ `VERIFIED` means.

```
$ esbmc b4_vector.cpp        # std::vector<int> v{1,2,3}; assume i<4; return v[i];
State 14  i = 3
Violated property:
  file /var/folders/.../T/esbmc-cpp-headers-c821-6846-be86/vector line 673
       column 5 function operator[]
  assertion operator[]
  !((_Bool)((signed long int)(!(i::0 < this->_size))))
VERIFICATION FAILED                                            0.26 s
```

The counterexample cites a `vector` in a **temporary directory ESBMC created**,
not libc++. ESBMC compiles C++ with `-nostdinc++` and substitutes its own
*operational models* (OMs). I captured the directory mid-run: **67 modelled
headers**, including `vector`, `map`, `string`, `set`, `list`, `deque`,
`optional`, `memory`, `algorithm`, plus `boost/`, `Qt/`, `CUDA/` and
`systemc/` subtrees.

The opt-out exists and was measured:

```
$ esbmc b4_vector.cpp --no-abstracted-cpp-includes
/Library/.../c++/v1/__string/extern_template_lists.h:32: ...
/Library/.../c++/v1/__memory/compressed_pair.h:75: initializer of
  '__is_reference_or_unpadded_object<...>' is not a constant expression
fatal error: too many errors emitted, stopping now
ERROR: PARSING ERROR                                            0.35 s
```

**Pointed at the real libc++, ESBMC cannot parse it at all.** So there is no
choice to make: C++ verification here is verification against a substitute
library. ESBMC's own documentation states the models are "deliberately
simplified abstractions of the real library (#965)... written for verification
tractability, so their performance characteristics and internal
representations do not match a production standard library."

**Consequence for the spec.** Any row whose evidence is an ESBMC result over an
STL type is a claim about ESBMC's model of that type. If the model is a
*superset* of real behaviour the result is sound (`rust_adapter.md` §5.8's
over-approximation rule); if it is a subset it is not, and nobody has proved
which. This must be declared in the row's `Free:` cell, exactly like a stub.
It is not a reason to avoid the stage — it is a reason to write it down.

### 5.7 C++ specific: the stage 2 → stage 3 handoff, measured

ESBMC accepts loop invariants directly. `rust_adapter.md` could not demonstrate
this because Kani has no equivalent input. The chain works, and getting it to
work took four wrong attempts worth recording.

**Syntax.** The annotation goes **before** the loop, terminated with a
semicolon, and is a no-op without a flag:

```c++
__ESBMC_loop_invariant(s == 2*i);
while (i < n) { i++; s += 2; }
```

Measured failures on the way: placing it inside the loop header without a
semicolon → `error: expected ';' after expression`; placing it inside the body
→ parses but the annotation is ignored and the run times out;
`extern "C"` in a `.c` file → parse error. The intrinsic must bind to the
immediately following loop.

**Two flags, and they behave differently.** ESBMC has `--loop-invariant`
(combined mode: inductivity check + k-induction) and `--loop-invariant-check`
(legacy three-part havoc abstraction). Measured on the same file:

| case | `--loop-invariant` | `--loop-invariant-check` |
|---|---|---|
| `s == 2*i`, assert `s == 2*i` | SUCCESSFUL, "inductive step (k = 2)", 0.16 s | SUCCESSFUL |
| `2*i == seen && i <= n`, assert `seen == 2*n` | **UNKNOWN**, 1.34 s | **SUCCESSFUL, 0.17 s** |
| LoopInvGen's verbatim output, assert `seen == 2*n` | **UNKNOWN**, 0.77 s | **SUCCESSFUL, 0.16 s** |

**Use `--loop-invariant-check`.** The combined mode returned UNKNOWN on every
invariant strong enough to prove a post-loop property about `n`, including
correct ones, and adding `__ESBMC_loop_assigns(i, seen)` did not help. The
legacy mode discharged all three.

**The end-to-end result.** LoopInvGen's output for `detect_scan` was
transliterated mechanically from SyGuS to C++ — no human strengthening:

```c++
__ESBMC_loop_invariant(((!(i >= n)) || (seen == 2*n)) && (2*i == seen));
while (i < n) { i++; seen += 2; }
__ESBMC_assert(seen == 2*n, "detect_scan returns 2n");
```

`esbmc e2e_pipeline.cpp --loop-invariant-check --overflow-check` →
**VERIFICATION SUCCESSFUL in 0.16 s**, for all `n <= 65536`, on a loop that
plain BMC unwound 4.5 million times without an answer (§5.4).

**The mechanism cannot be cheated.** Two negative controls:

| invariant | true? | strong enough? | result |
|---|---|---|---|
| `s == 3*i` | **no** | — | `Violated property: loop invariant inductive step` → **FAILED** |
| `s >= 0` | yes | **no** | **FAILED** (legacy) / diverges through k=4+ (combined) |
| `s == 2*i` | yes | yes | **SUCCESSFUL** |

A wrong invariant is rejected as non-inductive; a true-but-weak one does not
silently pass. That is what makes stage 2's output admissible as evidence
rather than as a hint.

### 5.8 C++ specific: what the language does and does not break

Every row measured on this machine, ESBMC 8.4.0.

| feature | result | time |
|---|---|---|
| **templates** (`clamp_add<int32_t>`) | SUCCESSFUL, verified through the monomorphization | 0.19 s |
| **recursive template metaprogramming** (`Fact<5>::v == 120`) | SUCCESSFUL | 0.20 s |
| **virtual dispatch** (`Base*` → `D1`/`D2`, assert `f() != 0`) | **FAILED, correct arm** — counterexample picks `D2`, `p = &dynamic_2_value`, `v = 0` | 0.17 s |
| **exceptions, caught** (`throw int` / `catch (int e)`) | SUCCESSFUL, 7 VCCs | 0.16 s |
| **exceptions, uncaught** | **FAILED — `uncaught exception`** | 0.15 s |
| **`std::vector` OOB** | FAILED, correct index (i=3, size 3) | 0.26 s |
| **`std::string` OOB** | FAILED — `Error! Invalid access memory area` in modelled `string:1438` | 0.31 s |
| **`std::unique_ptr` null deref** | FAILED — `dereference failure: NULL pointer`, CWE-476 | 0.42 s |
| **`std::map`** | **ERROR: Timed out** at 60 s | 61.29 s |
| **`std::map` with `--incremental-bmc`** | **FAILED**, correct counterexample (`m[2]` defaults to 0, not 10) | **0.62 s** |

Templates, virtual dispatch, and exceptions — the three features usually named
as C++ blockers — all worked, and the virtual-dispatch counterexample resolved
to the *correct* override. The blocker is **associative containers**, and even
that has a flag: `--incremental-bmc` converted a 60 s timeout into a 0.62 s
correct answer, a **99× speedup**. That is the single highest-value flag
found in this session and it belongs in the default portfolio.

ESBMC's own limitations page corroborates and extends this: constructor and
destructor *ordering* is not correct in every case (#940) and "a program whose
correctness depends on one of these orderings may verify when it should not";
nested `std::vector::insert` does not converge; `<forward_list>`, `<regex>`
and others have **no model at all**. Not measured here — cited, and labelled.

### 5.9 Generics: the same open problem as Rust, with less help

`clamp_add<int32_t>` verified because the harness named `int32_t`. libclang
reports `template_params=['T']` (§3.2) and nothing else. A template is not a
domain; it needs a **witness type per parameter**, and C++ gives the adapter
less to work with than Rust does — there is no trait-bound list to map from.
Pre-C++20 the constraints are not in the type system at all; with `concepts`
(which ESBMC supports) there is something to read, but the subject here has
none.

The adapter owns a constraint→witness-type table, and for unconstrained `T` the
honest answer is that the table is a **stage-1 heuristic**: pick the smallest
primitive that admits the operations used in the body. **Not built, not
measured.** The Rust adapter has the same gap (§5.6 there) and the
verify-rust-std effort has no better answer either.

### 5.10 CBMC is installed and is not usable for C++ on this machine

`brew install cbmc` → 6.11.0, 44 files, 164.7 MB. Measured:

| input | result |
|---|---|
| C++ with **no** STL headers, unsigned+signed subtraction | **works** — both overflows found, `[seek_current.overflow.1]` and `.2`, **0.02 s** |
| C++ with `#include <cstdint>` | `PARSING ERROR` — parse errors inside `cstdint`, `enable_if.h`, `integral_constant.h`, `remove_cv.h` |
| C++ with `#include <vector>` | `PARSING ERROR` (also via `goto-cc`) |
| C++ classes + virtual dispatch, no STL | **crash** — `--- begin invariant violation report --- Invariant check failed, File: src/goto-symex/goto_symex.cpp:53, Condition: lhs.type() == rhs.type()` with a backtrace |

CBMC is *fast* on the fragment it accepts — 0.02 s versus ESBMC's 0.26 s on the
same arithmetic — but it cannot parse any modern standard-library header and it
aborts internally on ordinary virtual dispatch that ESBMC handles in 0.17 s.

This is not a local misconfiguration. CBMC issue **#6735** records a maintainer
response to the identical macOS libc++ parse failure:

> The C++ front-end only handles basic uses of templates... Modern STL
> implementations use a *lot* of template meta-programming and so the C++
> doesn't handle them.

and, on the prospect of fixing it:

> Alternative approaches would be using the clang front-end like ESBMC does...
> If someone was paying me to support C++... I think I might take an approach
> like... a minimal / partial "verification friendly" STL implementation
> rather than try to parse "real" ones.

That last sentence describes precisely what ESBMC's 67 operational models are
(§5.6). Issue #5489 was closed with "improvements to the C++ frontend... are
very far off from our end."

**Decision: ESBMC is the C++ prove-stage backend. CBMC is not a portfolio
member for C++.** It remains the right tool for C, and it is what Kani uses
under Rust — a useful reminder that Kani's C++ story does not exist either.

### 5.11 Cost scaling

Array fill, `--unwind N+2`, assert one element:

| N | VCCs | time | verdict |
|---|---|---|---|
| 10 | 23 | 0.21 s | SUCCESSFUL |
| 50 | 103 | 0.20 s | SUCCESSFUL |
| 100 | 203 | 0.24 s | SUCCESSFUL |
| 500 | 1,003 | 8.18 s | SUCCESSFUL |
| 1,000 | 2,003 | 46.42 s | SUCCESSFUL |
| 2,000 | 4,003 | **timeout at 200 s** | — |

VCC count grows **linearly** (2N+3); solve time does not. 100 → 1,000 is a 10×
size increase and a **193×** time increase. The practical ceiling for a
straight-line bounded proof on this machine is around N≈1000.

This is why §5.12 exists: the answer to a big bound is not a bigger timeout.

### 5.12 Contracts — the scaling mechanism

Same idea as `rust_adapter.md` §5.8, and it works in ESBMC. Clauses go
**inside** the function body (measured — putting them after the declarator is
`error: expected function body after function declarator`):

```c++
unsigned long scratch_len(unsigned long d) {
    __ESBMC_requires(d <= CAP);
    __ESBMC_ensures(__ESBMC_return_value <= STEP &&
                    __ESBMC_return_value + d <= CAP);
    if (d >= CAP) return 0;
    unsigned long r = CAP - d;
    return r < STEP ? r : STEP;
}
```

| mode | command | result | time |
|---|---|---|---|
| **enforce** | `--enforce-contract scratch_len --function scratch_len` | **SUCCESSFUL**, 2 VCCs | **0.19 s** |
| enforce, body broken (`STEP + 1`) | same | **FAILED** | — |
| **replace** | `--replace-call-with-contract scratch_len` | **SUCCESSFUL**, 1 VCC, body never unrolled | **0.18 s** |

The whole domain `0..=65536` discharged in 0.19 s, versus §5.11 where a
straight bounded proof over 1,000 elements took 46 s. **Always pair
`--enforce-contract f` with `--function f`** — without it ESBMC follows the
call chain from `main` and the function sees only the inputs `main` produces.

Replace mode is an over-approximation and therefore sound (ESBMC's theory page
states this explicitly); a too-weak contract costs precision, not soundness.
Same rule as Rust: contracts are sound by construction, raw stubs are not.

### 5.13 Flag recipe

```sh
# default per-function harness
esbmc <f>.cpp --overflow-check --unsigned-overflow-check \
              --k-induction --timeout 90s

# if k-induction returns UNKNOWN and stage 2 produced an invariant
esbmc <f>.cpp --overflow-check --unsigned-overflow-check \
              --loop-invariant-check --timeout 90s

# if a container makes it diverge
esbmc <f>.cpp --overflow-check --unsigned-overflow-check \
              --incremental-bmc --timeout 90s

# only with a bound cited from a constant in the patch
esbmc <f>.cpp --overflow-check --unsigned-overflow-check --unwind <N>

# NEVER
#   --no-unwinding-assertions       (unsound; hides a too-small bound)
#   (bare esbmc with no check flags — see 5.1, reports SUCCESSFUL on 0 VCCs)
```

Escalation order when a run does not terminate:
1. `--incremental-bmc` (measured 95× on `std::map`)
2. `--k-induction`
3. stage 2 invariant + `--loop-invariant-check`
4. `--replace-call-with-contract` on the expensive callee
5. `--unwindset L:n` per loop (`--show-loops` for labels)

---

## 6. Stage 4 — ADEQUACY (is the spec strong enough)

This is the false-positive direction of `finalarchitecture.md` §1.2.

**Mutate the code, re-run the proof — not the tests.**

- proof still passes on a mutant → mutant **live** → the spec is too weak; a
  wrong implementation could pass. Missing row.
- proof fails → mutant **killed** → that spec row is doing work.

### 6.1 No off-the-shelf C++ mutation tool installed

Recorded as a reproducible blocker, not glossed:

- `brew install mull` → no such formula (Homebrew suggests `mullvad-browser`).
- `brew tap mull-project/mull` → tapped, 18 formulae.
- `brew install mull-project/mull/mull` → `No available formula... Did you mean
  mull@21, mull@20, mull@19, mull@18, mull@17, mull@16?`
- `brew install mull-project/mull/mull@21` → **fails**:
  ```
  Error: Failed to download resource "mull@21 (0.30.0)"
  Download failed: https://dl.cloudsmith.io/public/mull-project/mull-stable/
                   raw/names/mull-21/versions/0.30.0/PACKAGE_FILENAME_PLACEHOLDER
  curl: (56) The requested URL returned error: 404
  ```
  The formula's URL contains the literal string `PACKAGE_FILENAME_PLACEHOLDER`
  — broken upstream, not a local problem.
- `docker pull ghcr.io/mull-project/mull:latest` → `denied` from the registry.

`mutate++` was not attempted after this. **Mull was never run.**

### 6.2 The harness that was built and run

This matters more than mull would have. Mull mutates and re-runs the **test
suite**; Hyperray needs mutate-and-re-run-the-**proof**, which no packaged tool
does. `/tmp/cppadapter/mutate_reprove.py` (70 lines) does it: 8 syntactic
operators (AOR `+`↔`-`, ROR `<`↔`<=`, `>=`→`>`, `==`→`!=`, constant
`0`↔`1`), one mutant per site, ESBMC re-run per mutant, `--ESBMC` lines never
mutated so the specification itself is never weakened.

**Subject**: `scratch_len`, the C++ analogue of the Rust contract case.

**Run 1 — weak spec** (one assertion: `r <= SCRATCH_STEP`):

```
baseline: SUCCESSFUL (0.52s)
mutants: 5 generated, 5 ran, 0 invalid
KILLED 0   LIVE 5   mutation score 0.0%
```

Every mutant survived, including `MAX_DETECTION_PREFIX_LEN - dst_len` mutated
to `+`. The proof passed on a program with the arithmetic reversed. **The spec
was not strong enough and stage 4 said so.**

**Run 2 — two rows added** (`r + d <= CAP`; `d >= CAP ? r == 0 : r > 0`):

```
baseline: SUCCESSFUL (0.18s)
KILLED 2   LIVE 3   mutation score 40.0%
```

0% → 40% by adding the two rows the first run's survivors pointed at. That is
the loop the adapter is supposed to run.

### 6.3 The surviving mutants are provably equivalent — and BMC can prove it

The three survivors of run 2 were suspicious rather than damning, so they were
checked by proof rather than by argument:

```c++
unsigned long orig(unsigned long d){ if(d>=CAP) return 0; ... r<STEP?r:STEP; }
unsigned long mut1(unsigned long d){ if(d> CAP) return 0; ... }   // >= -> >
unsigned long mut2(unsigned long d){ ... r<=STEP?r:STEP; }        // <  -> <=
__ESBMC_assert(orig(d)==mut1(d), "mut1 equivalent");
__ESBMC_assert(orig(d)==mut2(d), "mut2 equivalent");
```

**VERIFICATION SUCCESSFUL, 0.17 s.** Both are equivalent mutants for every
`d <= 65536`. (The third survivor mutates `main`'s `return 0`, not program
logic.) So the true mutation score of run 2 is **2/2 = 100%**, and the spec is
adequate for this function.

This is a genuinely better position than test-based mutation, and it is the
argument for doing stage 4 with a prover at all: **the equivalent-mutant
problem — undecidable in general, and the standard reason mutation scores are
untrustworthy — becomes a decidable query over a bounded domain.** Stage 4 can
therefore report a *classified* score (killed / live / provably-equivalent)
instead of a raw one. `rust_adapter.md` §6 quotes case studies where
specifications killed only 40% and 60% of mutants; some unknown fraction of
those survivors were equivalent and nobody could tell which.

### 6.4 Boundary

Unchanged, and restated so the adapter never oversteps it:

> Coverage, PICT, mutation testing, fuzzing, property-based testing... may
> find useful witnesses. **None can produce `VERIFIED`.**

Only the four Semantic-IR queries produce a verdict:

```text
EXISTS x,o: C(x,o) AND NOT R(x,o)                = UNSAT
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))       = UNSAT
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)     = UNSAT
T(C)                                             = true
```

Mutation supplies counterexamples that reveal a missing row. It never supplies
the all-clear. The equivalence proof in §6.3 is a *stage-3* result being used
to interpret a stage-4 signal, which is the correct direction.

**Cost.** 5 mutants × ~0.2 s ≈ 1.1 s wall for the whole run — cheap here only
because the subject is small. `rust_adapter.md` §6 cites 2369% overhead for
mutation versus 24% for Inductive Validity Cores; with §5.11's scaling that
ordering will bite on a real patch. Run IVC first if an ESBMC equivalent
exists — **not investigated, untested.**

---

## 7. Stage 5 — PLAN (not built)

Input: the extraction manifest, the invariants from stage 2, the discharged
obligations from stage 3, the surviving mutants from stage 4.
Output: `plan.json`. **No code is emitted at this stage.**

Identical in shape to `rust_adapter.md` §7; the prior art (Camunda
`api-test-generator`'s `path-analyser`/`materializer` split, Imandra's Region
Decomposition) is language-independent and is not repeated. Three jobs, in
order of importance:

1. **Admissibility.** A row is admissible iff it derives from something the
   code *declares*: a type, an enum variant, a public member, a **named**
   constant, or a stage-2 invariant. A fact obtained only by executing the
   reference is **not** admissible.
2. **Deduplication.** Merge rows a single case observes.
3. **Pruning.** Drop arms for operations with a proved-total `__ESBMC_ensures`;
   stage 3 already discharged them.

**Three C++-specific jobs stage 5 must also do**, each falling out of a
measurement above:

4. **Record the OM caveat.** Every row whose evidence came from an ESBMC run
   touching an STL type carries a `Free:` declaration naming the operational
   model (§5.6). This is mechanical: the counterexample path contains
   `esbmc-cpp-headers-*`, so the planner can detect it without judgement.
5. **Choose witness types for templates** (§5.9). Unsolved.
6. **Budget the solver.** §5.11 makes bound selection a *scheduling* decision:
   a row needing N=2000 straight-line unwinding does not fit, and must be
   re-planned as a contract (§5.12) or dropped. Stage 5 needs the cost model,
   not just the row list.

---

## 8. Stage 6 — EMIT (not built)

Reads `plan.json`. Writes **the test file and `instruction.md` from one pass**.

Prior art: **DScribe** (ICSE 2022) — one template carries both the test
skeleton and the documentation fragment, generated together:

> Test suites and documentation capture similar information despite serving
> distinct purposes. Such redundancy introduces the risk that the artifacts
> inconsistently capture specifications.

DScribe is Java-only; the idea transfers, the code does not. For C++ the
emitted artifact has one extra part Rust's does not: a test file *and* a
`main()` harness with `__ESBMC_assume` bounds, since ESBMC has no
`autoharness` equivalent — every harness in this document was hand-written.
That is a real gap versus Kani and it enlarges stage 6's job.

Constraints inherited from the platform: `instruction.md` is GitHub-issue
prose, no bullets, no headings, pure ASCII, ≤500 words. Word budget is a
*scheduling* constraint on stage 5.

---

## 9. Per-language surface

| stage | Rust | C++ (this doc) | Python | Go |
|---|---|---|---|---|
| extract | Charon / rustdoc JSON / MIR | **libclang Python bindings** (not `-ast-dump=json`, §3.1) | `ast`, `inspect` | `go/types` |
| bound | RustHorn → LoopInvGen | **LoopInvGen** (SyGuS is language-free); no C++→CHC front end | LoopInvGen | LoopInvGen |
| prove | Kani → CBMC | **ESBMC** (CBMC unusable, §5.10) | **untested** | **untested** |
| adequacy | cargo-mutants / IVC | **hand-rolled mutate+reprove** (mull unavailable, §6.1) | mutmut | go-mutesting |
| plan / emit | shared | shared | shared | shared |

Two rows differ from what `rust_adapter.md` §9 predicted for C++, and both were
measured: it listed `clang -ast-dump=json` for extract (354 MB, §3.1) and
"CBMC / ESBMC" for prove (CBMC cannot parse C++ headers, §5.10). This document
supersedes those two cells.

Stage 2 is confirmed fully language-independent: the same Docker image, the
same `.sl` file format, and the same two invariant shapes that worked for Rust
worked for C++ with no tool change.

Python and Go prove-stage tooling remains **unverified**.

---

## 10. Honest status

| stage | what was MEASURED (tool actually run) | what was NOT |
|---|---|---|
| **1 EXTRACT** | `clang -ast-dump=json` timed and sized (354 MB / 1.46 s; 1.0 MB without STL; filter breaks JSON on 2+ matches). `extract.py` via libclang run on the subject: 0.19 s, 6,850 B, both constants, 7 enum variants, all branches/loops/subscripts, both defect subtraction sites, 3 binding blind spots found and fixed. | libclang LibTooling (C++ API) not built. No `compile_commands.json` project tried — single file only. Not run on a real multi-file patch. |
| **2 BOUND** | LoopInvGen in Docker on **2** C++-shaped loops: `0 <= len <= cap` in 1.14 s; `2i = seen` equality in 1.41 s. `goto-synthesizer` run and **killed at 692 s** with no output on a trivial C loop. | Frama-C/WP + Frama-Clang **not installed, not run** — rejected on the maintainers' own documentation ("early stage", "very rudimentary" stdlib, C++11 target). SyGuS translation is **manual**; no automatic C++→CHC front end tested. Only 2 loops, both mine. |
| **3 PROVE** | ESBMC 8.4.0 on **~25 programs**. Default run reports SUCCESSFUL on **0 VCCs** (§5.1). Overflow split across two flags; both needed. Bugs found: unsigned underflow, signed overflow, array OOB, null deref, `vector` OOB, `string` OOB, `unique_ptr` null, uncaught exception, virtual dispatch (correct arm). Clean control passes. Unbounded loop: 4.5M unwinds / 95 s timeout; `--no-unwinding-assertions` produces an **unsound** pass. `--unwind 9` = derived bound, k-induction independently found k=9. Templates, recursive TMP, exceptions, virtual dispatch all work. `std::map` 61 s timeout → **0.62 s** with `--incremental-bmc`. Real libc++ → `PARSING ERROR`; 67 operational models captured. Contracts enforce+replace both work (0.19 s / 0.18 s). Scaling N=10→2000. CBMC 6.11.0 installed: works with no STL (0.02 s), `PARSING ERROR` on any STL header, **internal invariant-violation crash** on virtual dispatch. | Concurrency/pthreads **not tested**. `--k-induction-parallel`, `--termination`, `--falsification`, `--goto-contractor`, solver choice (`--z3`, `--boolector`) **not tested**. Multi-file / whole-patch verification **not attempted**. `__ESBMC_is_fresh` and quantifiers **not tested**. Constructor/destructor ordering bugs (#940) **cited, not reproduced**. `<regex>`, `<forward_list>` gaps **cited, not run**. goto-transcoder portfolio **not tested**. |
| **4 ADEQUACY** | Hand-rolled `mutate_reprove.py` run twice: **0/5 killed** on the weak spec, **2/5 killed** on the strengthened spec; all 3 survivors **proved equivalent** by ESBMC in 0.17 s, so the true score is 2/2. Mull install failure reproduced with the exact 404 URL containing `PACKAGE_FILENAME_PLACEHOLDER`; `ghcr.io` image `denied`. | **Mull never ran.** `mutate++` never attempted. Only 8 mutation operators, one subject, 5 mutants. No statement-deletion operator. IVC equivalent for ESBMC **not investigated**. Not wired to `spec.md` rows. |
| **5 PLAN** | — | **Not built.** Cost model (§7.6) not built. Witness-type table not built. |
| **6 EMIT** | — | **Not built.** No `autoharness` equivalent exists for ESBMC; all harnesses here were hand-written. |

**Headline results.** The C++ pipeline runs end to end on a small subject:
libclang extracts two constants → LoopInvGen synthesizes `2i = seen` from them
→ ESBMC discharges the post-loop property with that invariant in 0.16 s on a
loop it otherwise unwound 4.5 million times without answering. Separately, the
transliterated `ReplayReader` defect is found in 0.34 s with a concrete
counterexample and CWE tags.

**Headline caveats.** (a) ESBMC verifies its own 67-header substitute standard
library, not libc++, and pointing it at libc++ fails outright — every STL-touching
row inherits that caveat. (b) The default invocation reports `VERIFICATION
SUCCESSFUL` on zero verification conditions. (c) CBMC, the tool named in the
Rust document's forward-looking table, cannot parse C++ standard headers at all.

**Untested: everything outside the ~25 single-file programs in
`/tmp/cppadapter`.** No real C++ project, no build system, no multi-file patch,
no concurrency.

---

## 11. Standing rules

Inherited from `rust_adapter.md` §11, plus five C++-specific ones earned here.

- **Compiling is not verifying.** Report which one happened.
- **`Generated 0 VCC(s)` is not a proof.** ESBMC prints `VERIFICATION
  SUCCESSFUL` on an empty obligation set. Parse the VCC count; zero is an
  error. *(new — §5.1)*
- **Pass both overflow flags.** `--overflow-check` alone silently ignores
  unsigned underflow, which is the more common C++ defect. *(new — §5.2)*
- **Never emit `--no-unwinding-assertions`.** It converts "bound too small"
  into a clean pass over a prefix of the state space. *(new — §5.4)*
- **Every bound cites a constant in the patch.** An invented bound can hide a
  defect. Where k-induction terminates, prefer it and check the reported k
  against the arithmetic. *(§5.5 — measured agreement at k=9)*
- **An STL result is a result about ESBMC's model of the STL.** Declare the
  operational model in the row's `Free:` cell; the counterexample path names it
  (`esbmc-cpp-headers-*`), so this is mechanical. *(new — §5.6)*
- **A `false` invariant is a translation bug**, never a result.
- **A live mutant is a hypothesis, not a verdict.** Ask the prover whether it
  is equivalent before declaring a missing spec row — over a bounded domain
  that question is decidable. *(new — §6.3)*
- **Search before declaring a limitation.** Held again here: `std::map` looked
  like a hard blocker at 61 s and was a flag away from 0.62 s; the loop-invariant
  handoff looked unsupported and needed one documented flag
  (`--loop-invariant-check`) rather than the one the release notes advertise.
- **Measuring the reference and asserting what you observed produces unfair
  tests.** The contract is the filter.
