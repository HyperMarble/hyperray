# Python adapter

Status: **DESIGN — 2026-09-02**

The Python instantiation of the Hyperray language-adapter protocol
(`lang-adpaters/PROTOCOL.md`). It states which external tool fills which stage,
what each one was measured to do, and what remains unbuilt.

Every measurement in this document was produced on 2026-09-02 on macOS 27.0 /
arm64, against purpose-built Python targets in `/tmp/pyadapter` that mirror the
shapes of noodles-296 (a prefix buffer, a byte-capped detection loop, and a
seek whose `Current` arm does arithmetic). Nothing here is cited from memory.
Where a claim is untested, it says so.

`rust_adapter.md` §9 says: *"Do not copy this document into `python-adapter.md`
without running the tools first."* Every tool below was installed and run. Three
of them behaved worse than their documentation promises, and those failures are
reported in §4.2, §5.8 and §6.4 because they change the design.

Toolchain measured (`uv pip list`, exact versions):

| tool | version | venv |
|---|---|---|
| CPython | 3.12.12 (via `uv venv --python 3.12`) | `.venv` |
| crosshair-tool | 0.0.110 | `.venv` |
| z3-solver | 5.1.0.0 | `.venv` |
| hypothesis | 6.167.1 (+ `[cli]`, black 26.5.1) | `.venv` |
| mutmut | 3.7.0 | `.venv` |
| cosmic-ray | 8.7.0 | `.venv` |
| mypy | 1.5.0 | `.venv` |
| pytype | 2024.10.11 | `.venv` |
| deal / deal-solver | 4.24.6 / 0.1.2 | `.venv` |
| icontract | 2.7.3 | `.venv` |
| nagini | 1.3.1 (z3 4.16.0.0, Java 21.0.8) | `.venv-nagini` |
| icontract-hypothesis | 1.1.7 (hypothesis pinned 6.98.0) | `.venv-ich` |
| LoopInvGen | `padhi/loopinvgen:latest`, digest `sha256:78d7562a…5dc0` | Docker |

The host `python3` is 3.9.6 at `/usr/bin/python3` with no `pip` module and
PEP 668 restrictions. Every environment was built with `uv venv` / `uv pip
install`; the system interpreter was never modified. Total install time for the
main five-package set: **14.6 s**.

**Three venvs, not one, and that is a finding.** `uv pip install nagini`
downgraded `z3-solver` 5.1.0.0 → 4.16.0.0 and `mypy` 2.3.1 → 1.5.0 in the
shared venv. Nagini and CrossHair cannot share an environment; the adapter must
shell out to isolated venvs per prover, not import them.

---

## 1. What the adapter is for

Unchanged from `rust_adapter.md` §1, and restated because Python changes none
of it. Hyperray proves four things about one fixed bounded task
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
task-authored tests.

**What Python changes is stage 1, and it changes it badly.** Rust's adapter
derives 156 obligations from types alone, before reading a single function
body, because `rustc` has already resolved every type. Python has no compiler
to lean on. §3.4 measures exactly what that costs: on the same file with
annotations stripped, the typed domain yield goes from 9/14 parameters to
**0/14**, while structural facts (branches, loops, constants) survive intact.
The whole Python design follows from that one asymmetry.

---

## 2. Stage map

```
        solution.patch + base checkout
                    |
   [1] EXTRACT      |  ast + dis + typing.get_type_hints   (+ mypy/pytype)
                    v
             extraction manifest  (PROTOCOL fmt 1 or graph fmt 2)
                    |
   [2] BOUND        |  LoopInvGen  (SyGuS; language-independent)
                    v
             manifest + Invariants column filled
                    |
   [3] PROVE        |  CrossHair (symbolic, Z3)  |  Nagini (Viper, if annotated)
                    v
             counterexample or "Confirmed over all paths"
                    |
   [4] ADEQUACY     |  mutmut / cosmic-ray  + diffbehavior triage
                    v
             surviving mutants -> missing spec rows
                    |
   [5] PLAN         |  NOT BUILT
                    v
             plan.json: which rows get a test, which get a sentence
                    |
   [6] EMIT         |  NOT BUILT (crosshair cover / hypothesis ghostwriter
                    v             are partial, and unsound as oracles — §8)
             test.patch + instruction.md, from one source
```

Stages 1–4 are existing tools, measured below. Stages 5–6 do not exist for
Python either, and are the adapter's actual engineering work. §8 reports the
one genuine surprise: two tools *almost* implement stage 6, and both produce
assertions by the method `evidence-rule.md` forbids.

---

## 3. Stage 1 — EXTRACT

### 3.1 Primary: the standard library, and no third-party extractor

Rust's adapter says **"Do not hand-roll a rustc driver. Three teams already
maintain one."** Python inverts this. There is no Charon for Python and none is
needed: `ast`, `dis`, `inspect` and `typing` ship with the interpreter, are
version-locked to it, and cover the three things the manifest needs. The
extractor written for this document is **447 lines**
(`/tmp/pyadapter/extract.py`) and required no dependencies.

Three independent front ends, deliberately kept separate so the manifest can
record which one supplied each fact:

| front end | sees | executes the module? |
|---|---|---|
| `ast` | source structure: signatures, annotations as written, branch points, loops, literal constants, `raise` sites | no |
| `dis` | `co_consts`: peephole-folded constants, interned tuples/frozensets, branchy opcode counts | no (compiles only) |
| `typing.get_type_hints` | annotations resolved through `from __future__ import annotations`, aliases, forward refs | **yes** |

The third one imports the target. That is a side-effect risk the Rust path
never has, and it is why `extract.py` carries a `--no-import` flag. The first
two are enough to produce the structural manifest.

### 3.2 Measured — annotated target

Run: `python extract.py target_annotated.py` on a 96-line module
(`Format` enum, `scratch_len`, `detect_format`, a `ReplayReader` class,
`classify`).

Extracted, exactly:

- **module constants**: `MAX_DETECTION_PREFIX_LEN = 65536:int`,
  `SCRATCH_STEP = 8192:int`, `Format = enum['BAM','CRAM','VCF']`
- **6 functions**, params annotated **11/14**, returns annotated **6/6**
- params carrying a finite domain from the type table: **9/14**
- **31 branch points** across 5 functions, each with line, kind and arm count
- **3 loops**, each tagged with whether its bound is readable from the source

Per-function branch and loop counts, as reported:

| function | branch points | loops | `bound_readable_from_source` |
|---|---|---|---|
| `scratch_len` | 2 | 0 | — |
| `detect_format` | 14 | 2 | `while` **false**, `for range()` **true** |
| `ReplayReader.__init__` | 0 | 0 | — |
| `ReplayReader.start_seek` | 2 | 0 | — |
| `ReplayReader.read_more` | 6 | 1 | `while` **false** |
| `classify` | 7 | 0 | — |

That `bound_readable_from_source` column is the stage-1→stage-2 handoff: the
two `while` loops marked **false** are exactly the two problems handed to
LoopInvGen in §4, and the `for i in range(len(prefix))` marked **true** needs
no invariant search at all.

`classify` also yielded `raises: ['DetectionError("unknown format")']` — a
declared failure mode, admissible as a spec row because the code names it.

### 3.3 Type → domain table

The Python analogue of `rust_adapter.md` §3.2. This is the `Parameters:` line
of a spec row, and it is **only reachable when an annotation exists**.

| type | finite domain values |
|---|---|
| `int` | `0`, `1`, `-1`, `sys.maxsize`, `-sys.maxsize-1` |
| `bool` | `True`, `False` |
| `float` | `0.0`, `-0.0`, `1.0`, `inf`, `nan` |
| `str` | `''`, `'a'`, lone surrogate `'\udcff'` |
| `bytes` / `bytearray` | `b''`, `b'\x00'`, `b'\xff'*N` |
| `list` / `tuple` / `set` | empty, one, many |
| `dict` | `{}`, `{k: v}` |
| `Optional[T]` | `None` + domain of `T` |
| `enum.Enum` | one value per variant |

Two entries differ from Rust in kind, not degree:

- **`int` has no min/max.** Rust's table opens with *"min, max, one past each
  (overflow witness)"*. Python integers are arbitrary-precision, so there is no
  overflow witness to enumerate and `sys.maxsize` is a machine-word artefact,
  not a type bound. The obligation that Rust discharges as "attempt to subtract
  with overflow" reappears in Python as an unbounded negative value — which is
  why §5.1's `unread_prefix` bug is a *sign* violation, not a panic.
- **`float` must include `nan`.** Measured in §5.3: CrossHair falsified
  `min(a,b) <= (a+b)/2 <= max(a,b)` with `float_avg(0.0, nan)`. `nan` is in the
  domain of `float` and breaks every ordering postcondition written without it.

### 3.4 Measured — the cost of no annotations

The control. `target_unannotated.py` is byte-for-byte the same code with
annotations removed and nothing else changed.

| extracted fact | annotated | unannotated | survives? |
|---|---|---|---|
| params annotated | 11/14 | **0/14** | **no** |
| returns annotated | 6/6 | **0/6** | **no** |
| params with a finite domain | 9/14 | **0/14** | **no** |
| module constants + enum variants | 3 | 3 | yes |
| functions with branch points | 5 | 5 | yes |
| branch points, `detect_format` | 14 | 14 | yes |
| loops detected | 3 | 3 | yes |
| `bound_readable_from_source` flags | 2 false / 1 true | 2 false / 1 true | yes |

**The structural half of stage 1 is annotation-independent. The typed half is
annotation-total.** Every domain in the manifest's `operations[].domains` and
every `graph.seeds` entry comes from the half that vanishes.

This is the honest statement of Python's core problem: Rust's adapter derives
156 obligations from `rustdoc` JSON *before looking at a single body*, because
the compiler already resolved every type. On unannotated Python that number is
**0**. Not "smaller" — zero. Branch and loop structure still arrives intact, so
the adapter is not blind, but it knows *where* the decisions are without
knowing *what values* reach them.

### 3.5 Measured — what recovers the type layer, and what it costs

Three candidates, all run.

**(a) mypy 1.5.0 — checks, does not infer.**

```
mypy --strict target_annotated.py    -> Success: no issues found in 1 source file
mypy --strict target_unannotated.py  -> 7 errors, all [no-untyped-def] / [no-untyped-call]
```

mypy tells you the annotations are *missing*. It does not supply them. Useful
as a gate ("this task is extractable"), useless as a recovery mechanism. Its
exit code is the mechanical admission test for the whole adapter: non-zero ⇒
the typed half of stage 1 will be empty (§3.4) **and** stage 3's verdicts will
degrade (§5.5).

**(b) pytype 2024.10.11 — infers returns, not parameters.**

`pytype target_unannotated.py` succeeded in **4.0 s** and emitted an inferred
`.pyi`. Counted mechanically from that stub:

- parameters given a non-`Any` type: **0/14**
- returns given a non-`Any` type: **2/6** (`classify -> str`,
  `detect_format -> Optional[Format]`)

It did recover the enum precisely (`BAM: Literal[1]`, `CRAM: Literal[2]`,
`VCF: Literal[3]`) and both module constants as `int`. Re-run in full
(non-`--quick`) mode via `pytype -k`: identical signatures. Re-run with a typed
caller module in the same invocation: `scratch_len(dst_len) -> Any`, unchanged.

So pytype recovers the *result* half of a small number of rows and **none of
the input domains**. Input domains are what the spec's `Parameters:` line
needs. pytype does not solve stage 1.

**(c) Runtime tracing — recovers everything, and is inadmissible.**

MonkeyType and pyannotate observe real calls and write annotations from what
they saw. This works, and the adapter **must not use it**.

`rust_adapter.md` §7.1 and §12 state the rule: *"A fact obtained only by
executing the reference and recording what happened is not admissible"* and
*"Measuring the reference and asserting what you observed produces unfair
tests. The reference answers more questions than the contract asks."* A traced
annotation is exactly that — `dst_len: int` inferred because the reference
happened to be called with ints. If the contract permits `float`, the traced
type silently narrows the domain and the generated tests never probe it: a
false negative in the sense of `finalarchitecture.md` §1.3.

Tracing is therefore ruled out **on protocol grounds, not capability grounds**.
It is the one technique that would close the gap in §3.4 and it is the one
technique the evidence rule forbids.

**Conclusion for stage 1.** The recovery order is: author annotations (full
yield, measured 9/14 domains) → pytype (returns only, measured 2/6, params
0/14) → nothing. There is no third option that respects the evidence rule.
**The adapter must gate on annotation coverage and declare a task
un-extractable below a threshold**, rather than silently producing a manifest
with empty domains.

### 3.6 Measured — `dis` sees constants the AST cannot

The Python analogue of `rust_adapter.md` §3.2's second dump. Run on a
purpose-built `folding.py`:

| source | `ast` reports | `dis` reports |
|---|---|---|
| `if n > 2 ** 13:` | `2:int`, `13:int` | **`8192:int`** |
| `return n - 1024 * 8` | `1024:int`, `8:int` | **`8192:int`** |
| `if x in (1, 2, 3):` | three separate ints | **`(1, 2, 3):tuple`** |
| `if x in {10, 20}:` | two separate ints | **`frozenset({10, 20})`** |
| `"a" "b" "c"` | `'abc':str` (parser folds) | `'abc':str` |
| module `CAP = 2 ** 16` | `2`, `16` | **`65536`** |
| module `MASK = 0xFF << 8` | `255`, `8` | **`65280`** |

`8192` is the real constant in `if n > 2 ** 13`. An adapter reading only the
AST would put `2` and `13` in the spec's `Evidence` cell — both wrong, and the
bound derived from them would be meaningless. This is the exact failure
`rust_adapter.md` §12 warns about: *"Every bound cites a constant in the patch.
An invented bound can hide a defect and produce a meaningless pass."*

`dis` also exposed structural facts the AST walk missed entirely: `match`
statements produce **7 branchy opcodes** in `matched()` while the AST visitor
recorded **0 branch points**, because `ast.Match` needs its own visitor method.
That is a real gap in the 447-line extractor, found by cross-checking the two
front ends against each other, and it is the argument for keeping both:
**opcode counts are a checksum on the AST walk.**

Grep targets for the bytecode pass: `POP_JUMP_IF_*`, `FOR_ITER`,
`JUMP_BACKWARD`, `MATCH_SEQUENCE`, `MATCH_MAPPING`, `MATCH_KEYS`, `COMPARE_OP`.

### 3.7 What extraction feeds

| manifest field (PROTOCOL) | source | annotation-dependent? |
|---|---|---|
| `operations[].domains` | §3.3 type table + enum variants | **yes — empty without annotations** |
| domain `source expression` | `ast` signatures + `get_type_hints` | **yes** |
| `graph.seeds` (fmt 2) | typed value domains | **yes** |
| `graph.transitions` | public functions from `ast` | no |
| constants in row outcomes | `dis` `co_consts` (§3.6) | no |
| branch/arm structure | `ast` visitor, checksummed by opcode counts | no |
| loop sites + bound readability | `ast` loop walk (§3.2) | no |
| declared failure modes | `ast` `raise` sites | no |

---

## 4. Stage 2 — BOUND (loop invariants)

`rust_adapter.md` §9 predicted this stage would port unchanged: *"Stage 2 is
fully language-independent: LoopInvGen consumes a SyGuS invariant problem, and
any front end that can emit `pre_fun` / `trans_fun` / `post_fun` gets the same
service."*

**Confirmed.** Same Docker image, same input format, four problems solved.

The prior art is unchanged and is restated only because it bounds what any tool
here can do: Karr 1976 (affine relations); Cousot & Halbwachs 1978
(inequalities, abstract interpretation); Rodríguez-Carbonell & Kapur 2004
(complete for polynomial equalities, at most `2m+1` iterations). Boundary: the
Monniaux Problem (JACM 2025) — undecidable for full polyhedra, polynomial-time
for a single affine loop.

### 4.1 Measured — the two data-dependent loops stage 1 flagged

Image pulled and run on macOS/arm64 (the image is linux/amd64 and runs under
emulation; Docker warns, and it works).

Invocation that works — the image's entrypoint is `opam config exec --`, so the
script must be named explicitly:

```sh
docker run --rm -v /tmp/pyadapter/sygus:/work \
  padhi/loopinvgen ./loopinvgen.sh /work/<problem>.sl        # synthesize
docker run --rm -v /tmp/pyadapter/sygus:/work \
  padhi/loopinvgen ./loopinvgen.sh -v /work/<problem>.sl     # synthesize + verify
```

Passing the `.sl` file as the bare argument produces **silent exit 0 with no
output** — the failure mode most likely to be mistaken for "the tool found
nothing." It is not; the entrypoint swallowed it.

| loop | source site | invariant found | time | `-v` |
|---|---|---|---|---|
| `detect_format` `while` | target:37 | `(or (not (<= want plen)) (<= plen 65536)) ∧ (plen >= 0)` | 1.4 s | PASS |
| `read_more` `while` | target:72 | `(not (<= want (+ -65536 total))) ∧ (total >= 0)` | 1.4 s | PASS |
| `for i in range(len(prefix))` | target:52 | `(<= i n) ∧ (i >= 0) ∧ (65536 >= n)` | 1.1 s | PASS |
| coupled counters (control) | — | **`(= (* 2 x) y)`** ∧ `x>=100 → y=200` | 1.0 s | PASS |

Both constants in the two real problems — `8192` and `65536` — were read from
the source by stage 1 (§3.2), not invented. That is `rust_adapter.md` §12's
rule, and it is mechanically enforceable here because §3.6 already proved the
extractor recovers folded constants correctly.

The `2x = y` control reproduces Rust's result exactly: an *equality between
variables*, Karr's 1976 class, outside the conjunctions-of-linear-inequalities
that simpler invariant searches cover. Same tool, same class, no Python-specific
loss.

### 4.2 Measured — an empty model reports `false`, and `-v` says PASS

`rust_adapter.md` §4 records a first attempt that returned `false` and calls it
a modelling error. That failure was reproduced deliberately, and it is worse
than the Rust write-up implies.

`read_more_bad_model.sl` over-constrains the precondition
(`total = 0 ∧ total >= 1` — unsatisfiable):

```
synthesize:  (define-fun inv_fun ((total Int) (want Int)) Bool false)
verify (-v): PASS
```

**`false` is a valid inductive invariant for an empty model, and the verifier
agrees.** The `-v` flag does not catch it. A pipeline that treats `-v … PASS`
as the acceptance signal will accept a vacuous invariant and carry it into the
spec's `Invariants` column, where it discharges every obligation for free.

**Rule, strengthened for this adapter**: a `false` invariant is a translation
bug, never a proof — *and `-v` PASS does not distinguish the two*. The adapter
must reject any synthesized invariant that is syntactically `false`, and should
additionally check `pre_fun` satisfiability before calling the tool at all.

### 4.3 Where the transition relation comes from

Hand-written today, exactly as in Rust. This is the stage's real gap.

Rust has a path out: RustHorn translates MIR into constrained Horn clauses,
and Kani builds the same relation internally. **Python has no equivalent, and
this is the largest unbuilt piece of the Python adapter.** Nothing in the
standard library produces a transition relation, and no third-party tool was
found that emits SyGuS from Python source.

What makes it tractable rather than hopeless: the four `.sl` files here are
20–26 lines each and mechanical in shape. Stage 1 already supplies every input
they need — loop site, guard expression, the variables mutated in the body, and
the constants (§3.6). The missing component is an `ast`-to-SyGuS emitter for
the affine fragment: assignments of the form `v = v ± const` and guards
comparing a variable to a variable or constant. That fragment covers all three
loops in the target file. Anything outside it (container mutation, method
calls, `enumerate` over a symbolic sequence) has no measured answer here.

Output lands in the spec's `Invariants` column.

---

## 5. Stage 3 — PROVE (CrossHair / Nagini)

```sh
uv venv --python 3.12 .venv && source .venv/bin/activate
uv pip install crosshair-tool        # pulls z3-solver 5.1.0.0
crosshair check <module-or-file-or-dir>
```

Install to first result: **14.6 s** for the whole five-package set. Compare
`cargo install --locked kani-verifier && kani setup` at ~2 min. Python's prove
stage has a dramatically lower setup cost, and — measured below — a dramatically
weaker guarantee.

Contracts are read from four sources (`--analysis_kind`): PEP316 docstrings
(`pre:` / `post:` / `__return__`), `icontract` decorators, `deal` decorators,
and bare `assert` statements. All four were exercised.

### 5.1 Measured — every planted defect found, control clean

`buggy.py`: four functions with one real defect each, plus one correct control.
Cache cleared (`rm -rf ~/.cache/crosshair`) before every timing.

| target | defect | CrossHair verdict | time |
|---|---|---|---|
| `scratch_len` | no lower clamp → negative result | `false when calling scratch_len(65537)` (returns `-1`) | 0.09 s |
| `unread_prefix` | `prefix_len - position` unguarded | `false when calling unread_prefix(0, 1)` (returns `-1`) | 0.09 s |
| `nth_or_last` | off-by-one, `i == len(xs)` indexes out of range | `IndexError when calling nth_or_last([0,0], 2)` | 0.10 s |
| `mean_of` | division by zero on empty list | `ZeroDivisionError when calling mean_of([])` | 0.08 s |
| `clamp` (control) | none | **`Confirmed over all paths.`** | 0.09 s |
| whole file, one pass | — | all 4, control silent | 0.15 s |

Exit codes are contract-grade and scriptable: `0` no counterexample, `1`
counterexample found, `2` other error. Verified: `clamp` → 0, `mean_of` → 1.

`unread_prefix` is the direct Python translation of the real Rust defect at
`builder.rs:322` (`rust_adapter.md` §5.3). Rust reports it as *"attempt to
subtract with overflow"* with counterexample `prefix_len=4, position=0,
offset=i64::MIN`. Python has no integer overflow, so the same code defect
surfaces only because a postcondition asserts `__return__ >= 0`. **Without a
written contract, this bug is invisible to every Python tool tested.** Rust
gets it free from the compiler's own inserted `assert`; Python requires the
author to state it. That is the §3 asymmetry reappearing in stage 3.

Also verified: with **no contract of any kind**, CrossHair reports
`WARNING: Targets found, but contain no checkable functions` and exits **0**
with zero analysis performed. Measured on the §3.2 target file. An adapter
reading exit codes alone cannot distinguish that from success.

### 5.2 Measured — "Confirmed over all paths" is a real verdict, and it is budgeted

`--report_all` distinguishes three outcomes, and the distinction is
load-bearing:

- `error: …` — counterexample found
- `info: Confirmed over all paths.` — path space exhausted, genuinely proved
- `info: Not confirmed.` — **budget exhausted, nothing proved**

The third is not a pass. Measured on `limits.loop_datadep`
(`while total < want: total += 8192`, `pre: 0 <= want <= 65536`):

| `--max_uninteresting_iterations` | verdict | time |
|---|---|---|
| 5 (default) | **Not confirmed.** | 0.95 s |
| 10 | Confirmed over all paths. | 0.12 s |
| 20 | Confirmed over all paths. | 0.12 s |
| 50 | Confirmed over all paths. | 0.13 s |

The default budget is too small for a loop bounded by a source constant, and
the difference between "not confirmed" and "confirmed" here is one CLI flag.
**An adapter that reads exit code 0 as success will silently accept
`Not confirmed`** — exit code is 0 in both cases, because no counterexample was
found. The adapter must parse `--report_all` output and treat `Not confirmed`
as *unproved*, never as *passed*.

This is the direct analogue of `rust_adapter.md` §5.4's finding that the bound
decides the answer. There the risk was an invented bound hiding a defect; here
it is an unstated budget silently downgrading a proof to a shrug.

### 5.3 Measured — where CrossHair stops

Seven probes, cache cleared each time.

| probe | outcome | time |
|---|---|---|
| `loop_datadep` — data-dependent trip count | Confirmed (at budget ≥10) | 0.12 s |
| `loop_datadep_buggy` — off-by-one in same loop | `false when calling …(1)` (returns 0) | 0.09 s |
| `uses_regex` — calls the C-implemented `re` module | **Confirmed over all paths** | 0.21 s |
| `deep_guard` — fails at one value behind `(n*7+13) % 999983` | **`false when calling deep_guard(571417)`** | 0.09 s |
| `str_slice` — unicode string indexing, off-by-one | `IndexError … str_slice('', 0)` | 0.10 s |
| `float_avg` — ordering postcondition on floats | **`false when calling float_avg(0.0, nan)`** | 0.11 s |
| `dict_merge` — symbolic dicts, real defect | **`Not confirmed`** — *missed* | 0.25 s |

Two results deserve emphasis.

**`deep_guard` is the case that justifies symbolic execution.** The
postcondition fails at exactly one input in `0..1000000`. CrossHair solved for
it in 0.09 s; the witness verifies (`(571417*7+13) % 999983 == 0`). §5.9
measures what random testing does on the same target.

**`dict_merge` is a genuine miss, not a budget problem.** The function returns
`a`-wins merge while the postcondition claims `b`-wins; `dict_merge({'x':1},
{'x':2})` returns `{'x': 1}`, violating it. CrossHair reported `Not confirmed`
at the default budget and **still `Not confirmed` after 36.7 s** at
`--per_condition_timeout 60 --max_uninteresting_iterations 300`. Symbolic
dictionaries with symbolic string keys are where this tool's model runs out.

`--analysis_kind=asserts` was also verified: on a file with no contract block
at all, only bare `assert` statements, CrossHair found both planted defects
(`AssertionError … scratch_len(65537)`, `IndexError … indexer([''], -2)`). That
matters for real code, which carries asserts more often than docstring
contracts.

### 5.4 Measured — `crosshair watch` works, including live re-analysis

`crosshair watch buggy.py` run as a background process against a copy:

- after 25 s: all four defects reported with **source context** (the offending
  line marked `>` amid surrounding lines), plus a live counter
  (`Analyzed 13/26/39/…/91 paths`)
- the file was then edited in place to fix `mean_of`
  (`return sum(xs)/len(xs) if xs else 0.0`)
- after 20 s more: **the `mean_of` report disappeared**; only lines 33 and 49
  (`unread_prefix`, `nth_or_last`) remained

So `watch` is a genuine incremental loop, not a re-run wrapper. For the adapter
this is a developer affordance, not a pipeline component — `check` with parsed
`--report_all` output is the batch interface.

### 5.5 Measured — CrossHair without annotations

The §3.4 control, repeated at the prove stage. Same four contracts,
annotations removed.

| function | annotated | unannotated |
|---|---|---|
| `scratch_len` | `false … (65537)` | `false … (65537)` — same |
| `nth_or_last` | `IndexError … ([0,0], 2)` | **`TypeError: SymbolicObject`** — internal failure |
| `mean_of` | `ZeroDivisionError … ([])` | `ZeroDivisionError … ('')` — **found a `str`** |
| `clamp` (control) | `Confirmed over all paths.` | **`Not confirmed.`** |

Three distinct degradations, all from the same cause — with no annotation,
CrossHair must guess the parameter type:

1. `nth_or_last` crashed inside CrossHair's own `builtinslib` (`raise
   TypeError(type(i))`), producing a stack trace rather than a verdict.
2. `mean_of` was falsified with `xs=''` — a *string*, not a list. The defect is
   real either way, but the witness is outside any sane reading of the
   contract, and as a spec row it would be nonsense.
3. **The control regressed from proved to unproved.** `clamp` is correct and
   CrossHair confirmed it when annotated; unannotated it can only shrug.

The pattern is consistent with §3.4: the structural work survives, the typed
work does not. **Annotations are not a nicety for the Python adapter; they are
the precondition for both stage 1's domains and stage 3's verdicts.**

### 5.6 Measured — Nagini verifies, and demands its own subset

Nagini (ETH Zurich, Viper/Silicon backend, Java 21 + z3 4.16.0.0) in an
isolated venv.

| program | result | time |
|---|---|---|
| `scratch_len` with `Requires(0<=d<=65536)` / `Ensures(0<=Result()<=8192)` | **Verification successful** | 4.3 s |
| same body, `Requires` widened to `<=131072` | **Verification failed** — `Postcondition … Assertion Result() >= 0 might not hold`, with `Branch conditions: (not remaining > 8192)` | 6.1 s |
| `read_more` loop **with** `Invariant(...)` lines | Verification successful | 5.5 s |
| `coupled_without_invariant` (`Ensures(Result()==200)`, no `Invariant`) | **Verification failed** | 4.3 s |
| `coupled_with_invariant` (adds `Invariant(2*x == y)`) | **Verification successful** | 4.3 s |

That last pair is the measured stage-2 → stage-3 link, and it is the strongest
architectural result in this document:

> Nagini **cannot** prove `Ensures(Result() == 200)` without the relational
> invariant `2*x == y`, and **can** prove it with the invariant present.
> LoopInvGen synthesized exactly `(= (* 2 x) y)` for that loop in **1.0 s**
> (§4.1).

Stage 2 is not an optional enrichment of the manifest. It produces the artefact
stage 3 requires, and both halves were measured on the same loop.

Nagini's failure output is also better than CrossHair's for spec work: it names
the failing assertion *and* the branch condition under which it fails, which
maps onto a spec row's `Evidence` cell directly.

**The cost, and it is severe.** Nagini rejected both targets from §3:

```
nagini target_annotated.py    -> Translation failed
                                 Type error: Encountered Any type.
                                 Type annotation missing? (io@110.0)
nagini target_unannotated.py  -> same error
```

The fully annotated file fails too — the `Any` comes from the `io` module's own
stubs. Nagini requires a restricted, totally-typed Python subset with
`nagini_contracts` imports, not merely annotated Python. It cannot be pointed
at existing code.

**Placement**: Nagini is a *second* prover for hand-written contract kernels
(the analogue of `rust_adapter.md` §5.7's `scratch_len` hoisted-decision
pattern), not a whole-patch verifier. CrossHair is the default because it runs
on real code.

### 5.7 Measured — icontract enforces at runtime and infers strategies

`icontract` 2.7.3 decorators, checked by running the code:

```
scratch_len(1024)        -> 8192
scratch_len(70000)       -> ViolationError: 0 <= dst_len <= 65536: dst_len was 70000
scratch_len_buggy(70000) -> ViolationError: result was -4464
```

Runtime enforcement only — no proof. Its value to the adapter is as a
*contract notation* CrossHair already reads (`--analysis_kind=icontract`,
verified working in §5.8).

`icontract-hypothesis` 1.1.7 does something no other tool here does:

```
infer_strategy(scratch_len)
  -> fixed_dictionaries({'dst_len': integers(min_value=0, max_value=65536)})
infer_strategy(scratch_len_buggy)
  -> fixed_dictionaries({'dst_len': integers(min_value=0, max_value=131072)})

test_with_inferred_strategy(scratch_len)        -> PASSED
test_with_inferred_strategy(scratch_len_buggy)  -> FAILED, ViolationError: result was -1
```

**It derives the strategy from the precondition, not the type.** Compare
ghostwriter (§5.9), which emits `st.integers()` — unbounded — from the `int`
annotation alone. The bound `0..65536` is exactly the kind of fact the spec's
`Parameters:` line needs, and it comes from a declared contract, so it is
admissible under the evidence rule.

**Compatibility failure, and it cost a corrupted measurement.** On the main
venv, `import icontract_hypothesis` raises:

```
AttributeError: module 'hypothesis.internal.reflection'
                has no attribute 'extract_lambda_source'
```

against hypothesis 6.167.1. It works on hypothesis **6.98.0** in a separate
venv. Because `icontract-hypothesis` registers a pytest plugin, this broken
import made **every** `pytest` invocation in that directory fail at collection
— which silently corrupted a cosmic-ray run (§6.4). Isolated venv required.

### 5.8 Measured — deal-solver produces false positives, and is excluded

`deal-solver` 0.1.2 (via `python -m deal prove`) is documented as proving
`deal` contracts with Z3. It ignored preconditions in every test.

```
deal_demo.py
  scratch_len          failed post-condition. Example: dst_len=65537.
  scratch_len_buggy    failed post-condition. Example: dst_len=65537.
```

`scratch_len` carries `@deal.pre(lambda dst_len: 0 <= dst_len <= 65536)`, and
`0 <= 65537 <= 65536` is false. The witness lies outside the stated
precondition. **The correct function was reported as failing.**

Reproduced on a two-line minimal file: `f` (with `@deal.pre(0<=x<=100)`) and
`g` (identical body, no precondition) both reported `failed post-condition.
Example: x=101.` The precondition *is* attached and *is* enforced at runtime —
`f(101)` raises `PreContractError: expected 0 <= x <= 100 (where x=101)` — so
this is a solver bug, not a missing decorator.

The controlled comparison, same contract shape, same defect, three tools:

| tool | correct function | buggy function |
|---|---|---|
| `crosshair --analysis_kind=icontract` | silent (correct) | flagged `scratch_len_buggy(65537)` |
| `crosshair --analysis_kind=deal` | silent (correct) | flagged `scratch_len_buggy(65537)` |
| `python -m deal prove` | **flagged (false positive)** | flagged |

CrossHair reads `deal` decorators correctly and honours the precondition;
`deal prove` does not. **Do not use deal-solver in this pipeline.** Use `deal`
or `icontract` as contract *notation* and CrossHair as the solver.

### 5.9 Measured — hypothesis, and why it is not the prover

`uv pip install hypothesis` (6.167.1). The CLI additionally needs
`hypothesis[cli]`, which pulls `black` — without it every `hypothesis write`
invocation fails with a message naming `pip` rather than `uv`.

**Ghostwriter output.** `hypothesis write target_annotated.scratch_len`
produces a runnable, correctly-annotated test:

```python
@given(dst_len=st.integers())
def test_fuzz_scratch_len(dst_len: int) -> None:
    target_annotated.scratch_len(dst_len=dst_len)
```

**No oracle.** The body calls the function and asserts nothing; it can only
catch an exception. The strategy is `st.integers()` — unbounded — because the
annotation says `int`. The source's own `0..65536` bound (§3.2) is not used.
Contrast `icontract-hypothesis` (§5.7), which produced
`integers(min_value=0, max_value=65536)` from the precondition.

On the **unannotated** version, same function:

```python
# TODO: replace st.nothing() with an appropriate strategy
@given(dst_len=st.nothing())
def test_fuzz_scratch_len(dst_len):
```

`st.nothing()` generates no values. The test is vacuous and says so. This is
the §3.4 asymmetry a third time: ghostwriter is a pure function of the type
annotations.

**Whole-module output does not run.** `hypothesis write target_annotated`
emitted `@given(src=st.from_type(_io.BufferedReader), …)` where `_io` is never
imported → `NameError: name '_io' is not defined` at pytest collection; the
entire file fails to import. After hand-inserting `import _io`, the run gives
**2 passed, 2 failed**:

- `test_fuzz_detect_format` — `ResolutionFailed: Could not resolve
  <class '_io.BufferedReader'> to a strategy`
- `test_fuzz_classify` — `DetectionError: unknown format`, falsified by
  `fmt=None, strict=True`

That second failure is **the tool flagging intended behaviour as a bug**.
`classify` is documented to raise `DetectionError` when `fmt is None and
strict`; §3.2's extractor recorded that `raise` site as a declared failure
mode. Ghostwriter has no way to know it is intended. `--except` fixes it, and
the fix must be supplied by the adapter:

```sh
hypothesis write target_annotated.classify --except target_annotated.DetectionError
```
→ wraps the call in `try/except DetectionError: reject()`.

Which means: **the declared-exceptions list from stage 1 is a required input to
any ghostwriter invocation**, or the generated suite reports false failures on
correct code.

**Random testing vs symbolic execution, same rare input.** Target `deep_guard`
(§5.3), one failing value in `0..1000000`, same machine:

| method | budget | result | time |
|---|---|---|---|
| CrossHair | default | **found** `n=571417` | **0.09 s** |
| hypothesis | 10 000 examples | **not found** (passed) | 2.1 s |
| hypothesis | 200 000 examples | found | 7.0 s |
| hypothesis | 200 000, 5 independent runs | **3 found / 2 missed** (22.1 s, 51.1 s, 32.5 s, 54.3 s, 34.1 s) | — |

Random property testing is **non-deterministic and unreliable** on this target:
2 of 5 runs at 200 000 examples reported a clean pass on code that provably
fails. CrossHair solves the constraint directly and is deterministic.

This is the measured, Python-specific instance of the boundary
`rust_adapter.md` §6 quotes from the spec skill:

> Coverage, PICT, mutation testing, fuzzing, property-based testing… may find
> useful witnesses. **None can produce `VERIFIED`.**

hypothesis is a witness generator for stage 4. It is not the prover.

**The one ghostwriter mode with a real oracle** is `--equivalent`, which
generates a differential test:

```python
@given(dst_len=st.integers())
def test_equivalent_scratch_len_ref_scratch_len_mut_const(dst_len: int) -> None:
    result_ref = diffpair.scratch_len_ref(dst_len=dst_len)
    result_mut = diffpair.scratch_len_mut_const(dst_len=dst_len)
    assert result_ref == result_mut, (result_ref, result_mut)
```

Run against three mutant pairs: ref vs `8193`-constant mutant → **1 failed**
(`assert 8192 == 8193`); ref vs `>=` boundary mutant → 1 passed; ref vs
hand-rewritten equivalent → 1 passed. The two passes are correct — both mutants
are behaviourally identical to the reference, verified exhaustively over
141 000 values in §6.2. So `--equivalent` is a usable stage-4 instrument, but
note what it *is*: a reference-versus-candidate comparison, which is the
adequacy question, not a specification.

### 5.10 Not measured in the prove layer

- **Nagini's Viper siblings** (Prusti, VerCors, Gobra): not run. Nagini itself
  is the Python front end and was measured; the others target other languages.
- **Z3 directly** as an SMT backend for a hand-built encoding: not attempted.
  CrossHair and Nagini both use it internally.
- **PyExZ3, PySym, angr**: not installed, not run.
- **CBMC/ESBMC on transpiled Python**: not attempted, and no reason to believe
  it is a path.
- **Patch-scale runtimes**: every measurement above is on files under 100
  lines. Nothing here predicts CrossHair's behaviour on a 14-file patch, which
  is the scale `rust_adapter.md` reports for Kani.

---

## 6. Stage 4 — ADEQUACY (is the spec strong enough)

This is the false-positive direction of `finalarchitecture.md` §1.2, and the
one place mutation belongs. The rule is unchanged from `rust_adapter.md` §6:

**Mutate the code, re-run the proof — not the tests.**

- proof still passes on a mutant → mutant **live** → spec too weak → missing row
- proof fails → mutant **killed** → that spec row is doing work

The published numbers remain sobering: two case studies' specifications killed
only **40%** and **60%** of mutants; a 1250-line SCADE cruise-control killed
**39%** while every line was marked covered. Inductive Validity Cores remain the
cheaper first pass (**24%** overhead vs **2369%** for mutation).

**What was measured here is the weaker form — mutate the code, re-run the
tests.** Wiring mutation to re-run the *proof* (§5.1) is not built.

### 6.1 Measured — mutmut 3.7.0 on a 17-statement module

Target `tiny.py` (`scratch_len`, `classify`, `clamp`). Config must use
`source_paths`; `paths_to_mutate` and `tests_dir` are deprecated and
`--no-progress` does not exist in 3.7.0.

| suite | line coverage | mutants | killed | survived | naive score |
|---|---|---|---|---|---|
| weak (smoke assertions only) | **94%** | 18 | 12 | **6** | 67% |
| strong (boundary assertions) | 100% | 18 | **15** | **3** | 83% |

Runtime: 0.66 s for 18 mutants (≈220 mutations/second). Reproduced twice.

The weak suite is the classic trap: **94% line coverage, 67% mutation score.**
Coverage said the code was exercised; mutation said a third of it was
unconstrained.

### 6.2 Measured — half the survivors were unkillable, and the true score is 100%

The six weak-suite survivors, each classified by exhaustive comparison against
the reference over the bounded domain (410 integer values for `scratch_len`;
all 729 triples in `[-4,4]³` for `clamp`, filtered to the documented
precondition `lo <= hi`):

| survivor | mutation | verdict |
|---|---|---|
| `scratch_len` #2 | `65536 - d` → `65536 + d` | **killable** at `d ∈ {57345, 65535, 65536}` |
| `scratch_len` #3 | `65536` → `65537` | **killable** at `d ∈ {57345, 65535, 65536}` |
| `scratch_len` #4 | `> 8192` → `>= 8192` | **EQUIVALENT** — unkillable |
| `scratch_len` #5 | `> 8192` → `> 8193` | **killable** at `d = 57343` |
| `clamp` #1 | `v < lo` → `v <= lo` | **EQUIVALENT under the contract** (36 differences exist, all with `lo > hi`) |
| `clamp` #2 | `v > hi` → `v >= hi` | **EQUIVALENT** — 0 differences anywhere |

**3 of 6 survivors cannot be killed by any test.** A mutation score that counts
them is wrong. Corrected: 15 killable mutants, and the strengthened suite killed
**15/15 = 100%**, with the 3 remaining survivors being exactly the 3 proved
equivalent.

The strong suite was not written by inspection. Every added assertion cites a
witness a tool produced — `dst_len=57343` and `57345` came from `crosshair
diffbehavior` (§6.3), the rest are boundary constants read from the source by
stage 1. No value was obtained by running the reference and recording the
answer; that is the §3.5 rule applied to test data.

### 6.3 Measured — `crosshair diffbehavior` triages survivors automatically

This is the piece that makes stage 4 mechanical instead of manual. Each mutmut
survivor was re-posed as a reference/candidate pair:

| candidate | diffbehavior | correct? |
|---|---|---|
| `65536 + d` | `dst_len=-57345`: 8192 vs 8191; `dst_len=57345`: 8191 vs 8192 | ✅ |
| `65537 - d` | `dst_len=57345`: 8191 vs 8192 | ✅ |
| `>= 8192` | *"No differences found… All paths exhausted, functions are likely the same!"* | ✅ **equivalent** |
| `> 8193` | `dst_len=57343`: 8192 vs 8193 | ✅ |
| `clamp` `v <= lo` | `v=0, lo=0, hi=-1`: -1 vs 0 | ⚠️ **witness violates `lo <= hi`** |
| `clamp` `v >= hi` | *"No differences found… likely the same!"* | ✅ **equivalent** |

Every run under 0.21 s. Four of six verdicts are exactly right, including both
"likely the same" calls — confirmed by exhaustive comparison, 0 differences over
141 000 values and 729 triples respectively.

**The one wrong verdict is instructive.** `diffbehavior` reported a difference
for `clamp` `v <= lo` at `(v=0, lo=0, hi=-1)`, but `lo <= hi` is the function's
stated precondition and `0 <= -1` is false. Exhaustive check *within* the
precondition: **0 differences** — the mutant is equivalent. `diffbehavior` does
not read the `pre:` block; it compares over the raw type domain.

**Rule**: `diffbehavior` output must be filtered against the precondition
before a witness becomes a spec row, or stage 4 will emit test cases the
contract never permits — the unfair-test failure mode of `rust_adapter.md` §12,
arrived at from a new direction.

### 6.4 Measured — cosmic-ray 8.7.0, and a corrupted run worth recording

Different operator set, larger mutant count, ~11 s per run.

| suite | mutants | survived | rate |
|---|---|---|---|
| weak | 55 | **15** | 27.3% |
| strong | 55 | **5** | 9.1% |

Operator breakdown: `ReplaceComparisonOperator` 29, `ReplaceBinaryOperator` 11,
`NumberReplacer` 10, `AddNot` 5. Roughly 3× mutmut's mutant count on identical
source — the two tools are not comparable by score, only by direction.

All 5 strong-suite survivors were decoded and checked exhaustively:
`classify` `n == 0` → `n <= 0` (**equivalent** — unreachable after `n < 0`),
`clamp` `<`→`<=` and `>`→`>=` (**equivalent under `lo <= hi`**), and
`scratch_len` `>`→`>=` (**equivalent**). Consistent with §6.2: after
strengthening, every remaining survivor is unkillable.

**The corrupted run.** The first cosmic-ray pair reported **0 survivors out of
55 for both suites** — including the weak one, which mutmut showed leaving 6
alive. The cause was the §5.7 plugin failure: with `icontract-hypothesis`
installed against an incompatible hypothesis, *every* `pytest` invocation died
at collection with `AttributeError`, so cosmic-ray's `test-command` returned
non-zero for every mutant and scored all 55 as killed. Diagnosed by applying one
mutant by hand and observing the suite fail for the wrong reason; uninstalling
the plugin and re-running produced the real 15/5 numbers above.

**Rule for the adapter, and it is not optional**: mutation testing must verify
that the unmutated baseline **passes** before trusting any score. A broken
harness reports a perfect score. mutmut is resistant to this because it runs its
own harness and checks a "forced fail test" first (visible in its output);
cosmic-ray took the `test-command` at its word.

### 6.5 The boundary

Restated from `rust_adapter.md` §6 because nothing in Python moves it. Only the
four Semantic-IR queries produce a verdict:

```text
EXISTS x,o: C(x,o) AND NOT R(x,o)                = UNSAT
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))       = UNSAT
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)     = UNSAT
T(C)                                             = true
```

Mutation supplies counterexamples that reveal a missing row. It never supplies
the all-clear. §5.9's measurement is the Python-specific reason to hold this
line: the same property-based tool passed and failed on identical correct-code
input across five runs.

---

## 7. Stage 5 — PLAN (not built)

Input: the extraction manifest, the invariants from stage 2, the discharged
obligations from stage 3, the surviving mutants from stage 4.
Output: `plan.json`. **No code is emitted at this stage.**

Prior art is unchanged from `rust_adapter.md` §7 — Camunda's
`api-test-generator` splitting `path-analyser` (plans, writes JSON only) from
`materializer` (the only stage that emits), and Imandra's Region Decomposition
for selection. Neither is Python-specific.

Three jobs, in order of importance:

1. **Admissibility.** A row is admissible iff it derives from something the
   code *declares*: a type annotation, an enum variant, a public item, a named
   constant recovered by `dis` (§3.6), a declared `raise` (§3.2), or a stage-2
   invariant. A fact obtained only by executing the reference and recording
   what happened is **not** admissible. Python-specific consequences, all
   measured:
   - a **traced** annotation (MonkeyType/pyannotate) is inadmissible — §3.5(c)
   - a `crosshair cover` assertion is inadmissible — §8.1
   - a `diffbehavior` witness must be filtered against the precondition — §6.3
2. **Deduplication.** Merge rows a single case observes. `Format` (3) ×
   `Optional` (2) is not always 6 cases.
3. **Pruning.** Drop failure arms already discharged by a stage-3 proof.

Python adds one job Rust does not have:

4. **Annotation gate.** §3.4 measured 0/14 typed domains on unannotated source
   and §5.5 measured the prove stage degrading from *proved* to *not confirmed*
   on the same code. The plan stage must refuse to emit domain-derived rows for
   unannotated parameters rather than emitting empty ones. `mypy --strict`'s
   exit code is the mechanical test.

---

## 8. Stage 6 — EMIT (not built)

Reads `plan.json`. Writes **the test file and `instruction.md` from one pass**.

Prior art: **DScribe** (ICSE 2022) — one template carries both the test
skeleton and the documentation fragment. Java-only; the idea transfers, the
code does not.

Constraints inherited from the platform: `instruction.md` is GitHub-issue
prose, no bullets, no headings, pure ASCII, ≤500 words. Word budget is a
*scheduling* constraint on stage 5, not a stage-6 formatting problem.

### 8.1 Measured — two tools nearly implement this stage, and both are unsound as oracles

This is the one place Python is genuinely ahead of Rust, and it is a trap.

**`crosshair cover` emits complete, runnable pytest files.**

```sh
crosshair cover --example_output_format=pytest diffpair.classify_ref
```
```python
from diffpair import classify_ref

def test_classify_ref():
    assert classify_ref(0) == 'zero'

def test_classify_ref_2():
    assert classify_ref(1) == 'pos'

def test_classify_ref_3():
    assert classify_ref(-1) == 'neg'
```

Three inputs, one per execution path, **with assertions** — path-derived, not
random. `--coverage_type PATH` on `scratch_len_ref` produced exactly the two
boundary-relevant inputs `57344` and `0`. Other formats: `eval_expression`
(default), `arg_dictionary` (`{"dst_len": 57344}` — directly consumable as
`plan.json` rows).

**Compare `rust_adapter.md` §12: "Kani emits a test only when it finds a
defect. A passing proof emits nothing."** CrossHair emits on success. That is a
real capability difference in Python's favour.

**And the assertions are inadmissible.** `assert classify_ref(0) == 'zero'` was
produced by *calling the reference with 0 and recording `'zero'`*. That is
precisely `evidence-rule.md`'s prohibition and `rust_adapter.md` §12's
*"Measuring the reference and asserting what you observed produces unfair
tests. The reference answers more questions than the contract asks."*

Measured proof that this is not theoretical — `crosshair cover` on the
unannotated `clamp` emitted:

```python
def test_clamp_untyped_2():
    assert clamp_untyped(0, 1, 0) == 1        # precondition is lo <= hi; 1 <= 0 is FALSE
def test_clamp_untyped_3():
    assert clamp_untyped(0, 0, -1) == -1      # 0 <= -1 is FALSE
```

**Two of three generated tests violate the function's own stated
precondition.** They pin behaviour the contract never promised. Committed as-is,
they would fail any correct reimplementation that rejects `lo > hi` — a false
negative in `finalarchitecture.md` §1.3's sense. (On the *annotated*
`buggy.clamp`, which carries a `pre:` block CrossHair parses, the generated
triples were all valid — so the failure tracks the missing contract, exactly as
§5.5 predicts.)

**hypothesis ghostwriter** is the other near-miss, measured in §5.9: it emits
runnable, correctly-annotated test files but **no oracle at all** in the default
mode, `st.nothing()` on unannotated code, a non-importing file for whole-module
input, and a false failure on an intended exception. Its `--equivalent` mode has
an oracle, and that oracle is a reference comparison — stage 4's question, not
stage 6's.

**Pynguin** (Fraunhofer / Passau; TOSEM 2022) is the third and the most
mature — an evolutionary test generator (DynaMOSA/MIO/MOSA) that emits full
pytest modules. **Not run in this session; read from its documentation only.**
Its own description of its oracle settles where it fits:

> Pynguin is able to generate *regression* assertions within its generated
> test cases based on the values that it observed during execution.

and of its inputs:

> the generated values are chosen randomly by Pynguin

Observed values as oracle is the `evidence-rule.md` prohibition verbatim;
random inputs are what stage 1 domains replace. Two things it does contribute:
`--assertion-generation NONE` gives a clean assertion-free driver (the
protocol's `driver` shape), and `--assertion-generation MUTATION_ANALYSIS`
keeps only assertions that kill mutants — stage 4's filter, already wired into
an emitter. Both untested here.

**Design conclusion for stage 6.** All three tools are usable as *input
generators* or *file writers* and none as an *oracle*:

- take `crosshair cover --example_output_format=arg_dictionary` output as
  candidate `plan.json` input rows — path-derived, deterministic, boundary-aware
- **discard every generated assertion**
- filter every candidate row against the precondition (§6.3, §8.1 both show
  unfiltered witnesses escaping the contract)
- have Hyperray supply the expected value per the protocol, from the spec row —
  not from a recorded observation
- emit the assertion and the `instruction.md` sentence from that single row, in
  one pass (DScribe's method)

The emit stage remains **not built**. What Python contributes that Rust does not
is a working, deterministic *input* generator for it.

---

## 9. Per-language surface

Updated from `rust_adapter.md` §9 with the Python column now measured.

| stage | Rust (measured 2026-09-01) | Python (measured 2026-09-02) |
|---|---|---|
| extract | Charon / rustdoc JSON / MIR | `ast` + `dis` + `typing`; **mypy gate, pytype partial** |
| bound | RustHorn → LoopInvGen | **LoopInvGen, same image, 4/4 solved** |
| bound: source→SyGuS | RustHorn / Kani internals | **nothing exists — largest gap (§4.3)** |
| prove | Kani → CBMC | **CrossHair (default), Nagini (contract kernels only)** |
| prove: obligations | compiler-inserted, free | **none — contracts must be written (§5.1)** |
| adequacy | cargo-mutants / IVC | **mutmut + cosmic-ray + `diffbehavior` triage** |
| plan / emit | shared, not built | shared, not built |

Stage 2 is confirmed fully language-independent: the same Docker image and the
same `.sl` format served both languages, and the `2x = y` control reproduced
identically.

Two structural differences that are not a matter of tool maturity:

1. **Rust gets proof obligations for free; Python does not.** `rustc` inserts
   `assert(…overflow)` into MIR, so stage 1 can *predict* which functions carry
   arithmetic obligations before stage 3 runs. Python's arbitrary-precision
   integers mean the equivalent defect (§5.1 `unread_prefix`) is invisible
   unless someone writes `post: __return__ >= 0`. **The adapter must author
   contracts for Python; for Rust it merely harvests them.**
2. **Python's prove stage is ~100× cheaper to install and weaker per run.**
   14.6 s vs ~2 min setup; 0.09 s vs 0.4–4 s per verdict. But Kani reports
   "580 checks passed" over a bounded scope, while CrossHair reports "Confirmed
   over all paths" only when the path space is exhausted within a budget the
   caller sets (§5.2), and returns exit 0 for `Not confirmed` too.

Untested here: C++, Go. `rust_adapter.md` §9's row for those stands unverified,
and this document does not change it.

---

## 10. The interpreter assumption

`rust_adapter.md` §10 records the compiler assumption: Kani proves at MIR level,
the verifier runs a compiled binary, and a miscompilation could separate the
two. Python has the same shape of gap, larger, and in a different place.

**CrossHair does not execute Python. It executes its own model of Python.**
Symbolic values are `SymbolicObject` proxies, and every builtin operation on
them is re-implemented in `crosshair/libimpl/builtinslib.py`. A proof is
therefore a statement about that library's semantics, not about CPython's.

This is not hypothetical, and it was observed directly. §5.5's unannotated
`nth_or_last` did not return a verdict — it raised from inside the model:

```
File ".../crosshair/libimpl/builtinslib.py", line 3985, in __getitem__
    raise TypeError(type(i))
```

The model ran out of cases. That instance was loud, which is the good outcome.
The `dict_merge` miss in §5.3 is the quiet one: a real postcondition violation
that the symbolic dictionary model never reached, reported as `Not confirmed`
rather than as an error.

Nagini has the same structure one layer further out: it translates a Python
subset into Viper, and the proof is about the Viper program. Its refusal to
translate ordinary annotated Python (§5.6, `Encountered Any type`) is that
translation being honest about its own limits.

Three reasons the gap is tolerable here, in increasing order of strength — the
first two are `rust_adapter.md` §10's arguments, which transfer unchanged:

1. **Hyperray compares; it does not ship.** The question is relative — does the
   submission behave like the reference over the bounded scope. Reference and
   submission cross the same interpreter, same version, same container. A model
   divergence applies to the analysis of both sides.
2. **`finalarchitecture.md` §1.4 catches the residue.** The reference must pass
   the verifier — real CPython, real bytecode, real machine. A model-to-runtime
   transfer failure surfaces there as a failing test, loudly.
3. **The residue is checkable per-run, cheaply, and Python makes it cheaper
   than Rust does.** seL4's answer to the compiler question was post-hoc
   translation validation rather than a verified compiler. The Python analogue
   is more direct: every counterexample CrossHair produces is a concrete input,
   so it can simply be **run against the real interpreter**. That was done for
   every counterexample in this document — `deep_guard(571417)` recomputed to
   `0` under CPython, `dict_merge({'x':1},{'x':2})` returned `{'x': 1}`,
   `str_slice('', 0)` raised a real `IndexError`. **A symbolic counterexample
   that does not reproduce concretely is a model bug, and checking costs one
   function call.**

That check is mandatory for this adapter and is not optional the way seL4's
binary validation was: it is a single execution per witness, and it converts
"CrossHair says" into "CPython does". No such cheap validation exists for a
*passing* proof — `Confirmed over all paths` remains a statement about the
model, and this document does not close that gap.

---

## 11. Honest status

Per-stage, with the distinction the task requires: **measured** means a tool was
installed and run on this machine and the number appears above.

| stage | tool | measured | NOT measured |
|---|---|---|---|
| **1 EXTRACT** | `ast`+`dis`+`typing` (447-line extractor) | 6 functions, 31 branch points, 3 loops, 11/14 annotated params, 9/14 typed domains; unannotated control **0/14**; `dis` recovered `8192` from `2**13`, tuple/frozenset consts | `match` statement branches (AST visitor gap, found via opcode checksum); decorators; `*args`/`**kwargs` domains; multi-file/patch-scale input; classes beyond one example |
| **1b type layer** | mypy 1.5.0, pytype 2024.10.11 | mypy clean / 7 errors; pytype **0/14 params, 2/6 returns**, 4.0 s, unchanged in full mode and with a typed caller | pyright, pyre; stub-file (`.pyi`) supplementation; typeshed coverage for third-party deps |
| **2 BOUND** | LoopInvGen (Docker) | **4/4 solved, all `-v` PASS**, 1.0–1.4 s; `2x=y` reproduced; `false`-invariant failure mode reproduced **and `-v` still PASSed** | **Python→SyGuS translation — hand-written, §4.3**; loops over containers; nested loops; `break`/`else` clauses |
| **3 PROVE** | CrossHair 0.0.110 | **4/4 planted defects found** (0.08–0.10 s), control `Confirmed over all paths`; budget sweep 5→10 flips *Not confirmed*→*Confirmed*; `deep_guard` needle in 0.09 s; `asserts` mode without contracts; `watch` incremental incl. live re-analysis; **unannotated: 1 internal `TypeError`, 1 nonsense witness, control regressed to *Not confirmed*** | multi-module analysis; class invariants; async/`await`; generators; C-extension-heavy deps beyond `re`; **patch-scale runtimes** |
| **3b PROVE** | Nagini 1.3.1 | 2 verified, 2 correctly rejected, 4.3–6.1 s; **`Ensures(Result()==200)` fails without `Invariant(2*x==y)` and succeeds with it** — the measured stage-2→3 link; **rejects ordinary annotated Python** (`Encountered Any type` from `io` stubs) | anything beyond 4 small programs; heap/aliasing (Viper's actual strength); IO; concurrency; Prusti/VerCors/Gobra |
| **3c** | deal-solver 0.1.2 | **false positive on correct code**, reproduced minimally; ignores `@deal.pre` while runtime enforcement works — **excluded from the pipeline** | whether a newer version fixes it |
| **3d** | icontract 2.7.3 + icontract-hypothesis 1.1.7 | runtime enforcement; **`infer_strategy` → `integers(0, 65536)` from the precondition**; correct pass/fail on the pair; **incompatible with hypothesis 6.167.1, works on 6.98.0** | `@invariant` on classes; inheritance / contract weakening |
| **3e** | hypothesis 6.167.1 | ghostwriter output for 3 modes; **whole-module file does not import** (`_io` NameError); false failure on an intended exception; `--equivalent` correctly 1 fail / 2 pass; **needle: 3/5 runs at 200k examples, 22–54 s vs CrossHair 0.09 s** | stateful / rule-based testing; `hypothesis.extra` beyond ghostwriter; targeted PBT |
| **4 ADEQUACY** | mutmut 3.7.0, cosmic-ray 8.7.0 | mutmut weak **12/18 (94% line coverage, 67% mutation)**, strong **15/18**; **3 of 6 survivors proved EQUIVALENT** exhaustively → true score **15/15**; cosmic-ray 55 mutants, weak 15 survive / strong 5 survive, all 5 equivalent; **`diffbehavior` triage 4/6 correct, 1 witness outside the precondition**; **a broken pytest plugin scored 55/55 "killed"** | **mutate-and-re-prove (the actual §6 rule) — not wired**; IVC for Python — no tool found or tried; scale beyond 17 statements |
| **5 PLAN** | — | nothing | **not built** |
| **6 EMIT** | `crosshair cover`, ghostwriter | 4 output formats incl. runnable pytest with assertions; path-derived boundary inputs; **2 of 3 generated tests violated the stated precondition** | **not built**; `instruction.md` generation entirely untried |

**Found by running the tools, contradicting their documentation or defaults —
each one changes the design:**

1. `deal prove` reports the **correct** function as failing, with a witness
   outside its own precondition (§5.8). Reproduced minimally. Tool excluded.
2. LoopInvGen synthesizes `false` for an empty model **and `-v` reports PASS**
   (§4.2). The verifier does not catch vacuity; the adapter must.
3. CrossHair returns **exit code 0 for `Not confirmed`** (§5.2), and exit 0 with
   a warning when there is no contract at all (§5.1). Exit code is not a proof
   signal; `--report_all` output must be parsed.
4. cosmic-ray scored **55/55 killed** when a broken pytest plugin made every run
   fail (§6.4). A baseline-passes check is mandatory.
5. `crosshair cover` emits tests that **violate the function's own
   precondition** when no contract is parseable (§8.1).
6. `hypothesis write <module>` emits a file that **does not import** (§5.9).
7. `diffbehavior` reports differences at inputs the **precondition excludes**
   (§6.3).
8. Installing `nagini` **silently downgraded z3 and mypy** in a shared venv.

**The one-line summary of Python versus Rust**: structural extraction is easy
and free (stdlib, no third-party extractor, 447 lines), invariant synthesis
ports unchanged (4/4, same image), the prove stage is cheap to install and found
every defect it was given — and all of it is contingent on type annotations that
the language does not require, with the one technique that would recover them
(runtime tracing) ruled out by the evidence rule rather than by capability.

---

## 12. Standing rules

Inherited from `rust_adapter.md` §12, plus the Python-specific ones this session
measured.

Inherited, unchanged:

- **Compiling is not verifying.** For Python: *importing is not verifying*, and
  *`Not confirmed` is not confirmed* (§5.2).
- **Every bound cites a constant in the patch.** §3.6 is why the `dis` pass is
  mandatory: `2 ** 13` in the AST is `8192` in the bytecode, and only one of
  those is the real bound.
- **A `false` invariant is a translation bug**, never a result — **and `-v`
  PASS does not detect it** (§4.2).
- **Search before declaring a limitation.**
- **Measuring the reference and asserting what you observed produces unfair
  tests.** Python has three new ways to violate this: traced annotations
  (§3.5c), `crosshair cover` assertions (§8.1), and unfiltered `diffbehavior`
  witnesses (§6.3).

Added for Python:

- **No annotations, no domains.** Measured 0/14 (§3.4), and the prove stage
  degrades from *proved* to *not confirmed* on the same code (§5.5). Gate with
  `mypy --strict` before building a manifest.
- **Contracts are authored, not harvested.** Rust's compiler inserts the
  obligations; Python's does not. A Python function with no contract yields
  `WARNING: Targets found, but contain no checkable functions` and exit 0
  (§5.1) — indistinguishable from success unless the output is parsed.
- **Run every symbolic counterexample against real CPython** before it becomes
  a spec row (§10). One function call converts "CrossHair says" into "CPython
  does"; a witness that does not reproduce is a model bug.
- **Never trust a mutation score without a passing baseline.** A broken harness
  reports 100% killed (§6.4).
- **Never trust a mutation score without equivalent-mutant triage.** Half the
  survivors here were unkillable (§6.2); `crosshair diffbehavior` does the
  triage in under 0.21 s (§6.3).
- **One venv per prover.** Nagini downgrades z3 and mypy; icontract-hypothesis
  pins hypothesis. Shell out to isolated environments; never import two provers
  into one interpreter.
- **`nan` is in the domain of `float`.** It falsified an ordering postcondition
  in 0.11 s (§5.3) and belongs in every float row of the domain table.
- **Filter every generated witness against the precondition** before it becomes
  a spec row. Three separate tools produced out-of-contract witnesses in this
  session (§5.8, §6.3, §8.1).
