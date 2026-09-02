# Hyperray design

Status: ACCEPTED 2026-09-02. This file replaces `pipeline-six-stages.md`,
`rust_adapter_plan.md`, and `docs/specs/{finalarchitecture,whole flow,
proof-requirements}.md`. `docs/specs/evidence-rule.md` stays. Per-language
measurements stay in `docs/<lang>_adapter.md`.

A stage is built from this file. It is done when its listed test passes on
every fixture. One passing case is not done (AGENTS.md rule 12).

## 1. What Hyperray does

Input: one `solution.patch` (the unified diff GitHub shows), the base
checkout it applies to, and the task text. One patch is one complete finite
bounded work. Many small bounded works sum into an open-ended system.

Output: `instruction.md` and a test file, written from one list of rows so
they cannot disagree.

Between them, seven stages. Stages 1-5 are per language, each adapter in its
own language. Stages 6-7 are one Go program shared by all four.

```
solution.patch + base + task text
  -> [1] EXTRACT   manifest.json      every changed function, placed by the compiler
  -> [2] SHAPE     shaped.patch       split under the rules; old = new proved per function
  -> [3] BOUND     manifest + bounds  one invariant per loop, each citing a constant
  -> [4] PROVE     proof.json         per function: proved, or one exact counterexample
  -> [5] ADEQUACY  proof.json + rows  mutate, re-prove; a survivor is a missing row
  -> [6] PLAN      plan.json          TTF partition rows from proof.json
  -> [7] EMIT      instruction.md + tests, from plan.json, in one pass
```

## 2. Rules every stage obeys

- **The tool tells us; we never guess.** Every fact about code comes from a
  compiler or a prover. An adapter holds no rule about how a language names
  or lays out things. (`rust_adapter.md` §3, measured: three hand-written
  naming rules failed on the second crate.)
- **Every number cites its line.** A bound, a domain, a constant carries
  `declared_by: file:line`. An invented number is refused.
- **`blocked` is an answer. Silence is a bug.** When a tool refuses, the
  stage reports the tool's own line, exactly.
- **Compiling is not verifying.** Both are reported, separately.
- **A `false` invariant is a translation bug, never a result.**
- **Evidence rule.** A row derived only by running the reference is
  inadmissible. The contract gives the row; the code gives its shape.
  (`evidence-rule.md`)
- **The task text is read in one place**: stage 5, at a surviving mutant,
  to name the row it reveals. Nowhere else.
- **Fixtures are tests.** They live outside the repository
  (`HYPERRAY_FIXTURES`, `HYPERRAY_FIXTURE_SRC`). No adapter knows a fixture
  name. A test asserts the rule, never the fixture's number.
- **Trusted base, stated.** The compiler and the prover are trusted. Every
  `proof.json` names both with versions. (Apple corecrypto states the same
  assumption; we copy the honesty, not the spec.)

## 3. Stage 1 — EXTRACT

Two passes.

**Pass 1** reads the patch text and nothing else: files, hunks, added and
removed ranges, the `fn` names the patch defines.

**Pass 2** opens only the files the patch names, takes the whole function
behind each hunk, then runs the language's own front end over the crate and
joins by file and line. Every file opened is recorded with its reason.

| language | front end | measured |
|---|---|---|
| Rust | Charon, whole crate, all features | noodles 4 crates 37 s; 5 async refused, named |
| C++ | libclang Python bindings (`cpp_adapter.md` §3.2) | 3 blind spots listed there |
| Go | `go/ast` + `go/types` + `x/tools/go/ssa` (`go_adapter.md` §3) | built and run |
| Python | `ast` + `dis` + `typing.get_type_hints` (`python_adapter.md` §3) | measured |

Output `manifest.json`: per function `{path, name, start_line, end_line,
text, status}` where status is `Extracted | Refused(reason) | Missing |
FileNotSeen`; per global `{path, line, source_text}`; `opened: [{path,
reason}]`.

**Test:** on every fixture with a source tree, every patched function has a
status, every refusal has a reason, no patched file is `FileNotSeen`, every
opened file is a patched file.

## 4. Stage 2 — SHAPE

Split every changed function under the ten rules (nesting 3, function 40
lines, file 75, no `unwrap`/`panic`, decisions return values). Then prove
old = new for each split function with the stage-4 tool, before anything
else runs on the new shape.

Measured once by hand (noodles, 2026-09-02): nesting 6 -> 2, Kani 9 min ->
3 s, one hidden subtraction became one named 5-line function, same 628
tests both ways, and the old line fails the same harness.

Output `shaped.patch` plus `equivalence.json`: per function
`{old, new, proved_equal: bool, tool, time_s}`. A function that cannot be
proved equal is not shaped; it goes forward as written and is marked.

**Test:** on every fixture, every function in `shaped.patch` has
`proved_equal: true` or is unchanged from the manifest; the shaped tree
passes the crate's own tests.

## 5. Stage 3 — BOUND

For every loop, one invariant that holds every turn, so the proof never
unrolls forever. Tool: LoopInvGen (SyGuS), language-independent, fed from
the front end's loop form (MIR for Rust, SSA for Go, libclang for C++, `ast`
for Python; `<lang>_adapter.md` §4 each).

A bound must be proved sufficient, not asserted: the prover's own unwinding
assertion, or k-induction where the loop is data-bounded.

Output: the manifest with an `invariants` column, each entry
`{loop_at: file:line, invariant, declared_by, tool, time_s}`.

**Test:** every loop in every changed function has an invariant or is
`blocked` with LoopInvGen's line; no invariant is `false`; no invariant
merely restates the loop guard.

## 6. Stage 4 — PROVE

Per function: proved, or one exact counterexample. Bounded model checking
against the safety obligations the compiler inserts (overflow, bounds,
unwrap, panic) plus every stage-3 invariant.

| language | tool | route on refusal |
|---|---|---|
| Rust | Kani autoharness + contracts | Charon `Coroutines are not supported` -> Kani `-Z async-lib` + `block_on`; anything else -> `blocked` |
| C++ | ESBMC 8.4.0, k-induction, contracts (`cpp_adapter.md` §5.13 flags) | no `<coroutine>` model -> `blocked` |
| Go | exhaustive enumeration over stage-1 finite domains; Gobra `--overflow` only for `forall` rows (`go_adapter.md` §5.7) | fuzz and `-race` are witnesses, never verdicts |
| Python | CrossHair (Z3); Nagini where annotated (`python_adapter.md` §5) | "Confirmed over all paths" is the verdict, and it is budgeted |

The four obligations, kept from the frozen v0.10 (`finalarchitecture.md` §5):

```
EXISTS x,o: C(x,o) AND NOT R(x,o)              = UNSAT   reference is right
EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))     = UNSAT   no false positive
EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)   = UNSAT   no false negative
T(C)                                           = true    reference is accepted
```

Output `proof.json`: header `{patch, base, compiler, prover, versions}`;
`facts: [{id, kind: bound|proved|counterexample|assumed, function, tool,
time_s, unwind, declared_by, inputs?}]`.

**Test:** on every fixture, every function has at least one fact; every
counterexample carries concrete inputs; every fact carries `tool` and
`declared_by`; the noodles fixture's known overflow appears as a
counterexample with its inputs (the one number a test may pin, because the
rule "a real defect is found" needs one real defect to check).

## 7. Stage 5 — ADEQUACY

Mutate the changed lines, re-run stage 4. A mutant the proof still passes
is live: nothing pinned that line. A live mutant is a question, not always
a hole — first prove equivalence (`go_adapter.md` §6.2: four of four
survivors were equivalent); if not equivalent, read the task text for that
line and write the row it reveals.

| language | mutation tool |
|---|---|
| Rust | cargo-mutants |
| C++ | source mutation + re-run ESBMC (70-line harness, `cpp_adapter.md` §6.2) |
| Go | gremlins + equivalence check |
| Python | mutmut, `crosshair diffbehavior` for triage |

Output: `proof.json` extended with `mutants: [{line, operator, status:
killed|equivalent|survived, row?}]`. Mutation never supplies the all-clear;
it supplies rows.

**Test:** on every fixture, every mutant on a changed line has a status;
every `survived` carries a row with a `declared_by`; no `survived` row was
derived by running the reference.

## 8. Stage 6 — PLAN (shared, Go)

Test Template Framework (Stocks & Carrington 1996), the method z-spec's
`partition` uses, fed from `proof.json` instead of a hand-written Z schema.
Nobody writes a spec; the rows are derived.

Per function: DNF over its branches, standard partitions and boundary
analysis on each stage-4 fact and stage-5 row. Row shape, from z-spec:

```
{id, class, branch, status: accepted|rejected, inputs, preState, postState,
 notes, declared_by}
```

Then the evidence rule filters: a row stays only if the contract owes it.

Output `plan.json`. **Test:** every row has a `declared_by` pointing into
the patch or a stage-5 row; no two rows share `inputs` and `branch`; every
counterexample from stage 4 became a rejected row.

## 9. Stage 7 — EMIT (shared, Go)

One pass over `plan.json` writes both files. Each row becomes one test in
the language's own test form and one sentence in `instruction.md`, in the
same order, with the same id. Neither file is edited after.

**Test:** the test file and `instruction.md` have the same row ids in the
same order; the emitted tests fail on every `survived` mutant from stage 5
and pass on the reference; the reference's own tests still pass.

## 10. Adapters

Each adapter is one binary in its own language, `adapters/<lang>/`, with
the same file names: `extract`, `shape`, `bound`, `prove`, `adequacy`. It
reads one JSON request on stdin and writes one JSON response on stdout;
`{"status":"blocked","blockers":[...]}` when it cannot finish. Tools are
found once at startup, version printed, pinned in the response.

The Go CLI runs the five per-language stages in order, then stages 6-7.

## 11. Status

| stage | Rust | C++ | Go | Python |
|---|---|---|---|---|
| 1 EXTRACT | done | not started | not started | not started |
| 2 SHAPE | not started | | | |
| 3 BOUND | not started | | | |
| 4 PROVE | not started | | | |
| 5 ADEQUACY | not started | | | |
| 6 PLAN | shared, not started | | | |
| 7 EMIT | shared, not started | | | |

Fixtures: Rust has four. The other three languages have none yet; each
needs two real upstream patches before "every fixture" means anything.
