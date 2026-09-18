# Stage 5 — ADEQUACY

Status: RESEARCHED 2026-09-03 against cargo-mutants 27.1.0 source and
two runs (docs/audit-2026-09-03.md). No code yet. Each phase gets a
**Proof** line the day it is built and its test passes on every fixture.

## What it is

Stage 4 proves a function cannot crash for every input in its bound.
Stage 5 asks whether every changed line is watched: it changes one line
of the patch on purpose, reruns the crate's tests, and checks whether
anything noticed. If nothing did, that line is pinned by nothing. That
is a hole, named by file and line, and stage 7 writes a test for it.

Measured 2026-09-03 on the two-function scratch crate with no tests:
14 mutants, 14 missed, 10 s — the correct answer for an untested patch.
On noodles v2 (`--list --in-diff`): **185 mutants** across four crates
in 1.2 s; genres FnValue 152, BinaryOperator 19, MatchArmGuard 8,
MatchArm 3, UnaryOperator 3.

## Input

- `proof.json` from stage 4: verdict per changed function.
- `solution.patch`: the diff whose lines are the only ones mutated.
- the crate's own tests, as they stand in the tree.

## Output

`adequacy.json`, the totals plus one row per mutant that survived:

```
{ "totals": { "total_mutants": 185, "caught": …, "missed": …,
              "unviable": …, "timeout": …, "success": true,
              "cargo_mutants_version": "27.1.0", "time_s": … },
  "survivors": [
    { "file": "noodles-util/src/alignment/async/io/indexed_reader/builder.rs",
      "package": "noodles-util",
      "span": { "start": { "line": 433, "column": 5 }, "end": { … } },
      "function": { "function_name": "relative_offset_behind_prefix",
                    "return_type": "-> io::Result<i64>",
                    "span": { … } },
      "genre": "BinaryOperator", "replacement": "+",
      "diff_path": "mutants.out/diff/…", "log_path": "mutants.out/log/…",
      "pinned_by": "nothing", "proof_verdict": "proved" } ] }
```

`pinned_by: nothing` is the row's whole meaning. `proof_verdict` is the
stage-4 verdict for the enclosing function, carried so a reader sees
that "proved" and "unpinned" are both true of the same line.

## Who knows what

| fact | source | how it is read |
|---|---|---|
| which lines to mutate | `cargo mutants --in-diff <patch>` (`-D`); the tool strips `b/` itself (in_diff.rs:197–200); a 4-crate git diff worked as-is | the tool's own flag |
| every mutant | `mutants.out/outcomes.json` → `outcomes[].scenario.Mutant` with `file`, `package`, `name`, `function {function_name, return_type, span}`, `span {start{line,column}, end{…}}`, `genre`, `replacement` | serde, fields from the measured file |
| what happened to each | `outcomes[].summary` — **six** values (outcome.rs:331): `Success`, `CaughtMutant`, `MissedMutant`, `Unviable`, `Failure`, `Timeout`. `Success` is the `Baseline` row's summary. | serde; all six named, a seventh fails to parse |
| the two scenario shapes | `scenario` is the string `"Baseline"` or the object `{"Mutant": …}` | untagged enum of two |
| the text change and the log | `outcomes[].diff_path`, `outcomes[].log_path` | kept verbatim |
| totals | top level: `total_mutants`, `caught`, `missed`, `unviable`, `timeout`, `success`, `start_time`, `end_time`, `cargo_mutants_version` | serde |
| the tool's exit | exit_code.rs: 0 Success, 1 Usage, 2 FoundProblems, 3 Timeout, 4 BaselineFailed, 5 FilterDiffMismatch, 6 FilterDiffInvalid, 70 Software | matched on the run record |

Facts not in the table are not read.

## Phases

### Phase A — run the tool on the patched lines

**Steps:**

1. From the tree root:
   `cargo mutants --in-diff <solution.patch> --all-features -o <work>
   --timeout-multiplier 3`. `--in-diff` is the tool's own scoping.
2. The tool runs the unmutated tree's tests first. Exit 4
   (`BaselineFailed`) → stop: the patch does not pass its own tests and
   stage 5 has nothing to measure; the row is the baseline log.
3. Exit 2 (`FoundProblems`) means mutants were missed; that is the
   expected result. Exit 5/6 mean the diff did not match the tree; that
   is a stage-1 fault and is reported as such.
4. Keep `exit_code`, stdout, stderr.

**Test:** on every fixture with a source tree, the run exits 0 or 2,
`outcomes.json` exists, and `total_mutants ≥ 1`.

**Proof:** (not built)

### Phase B — read `outcomes.json`

**Steps:**

1. Deserialize the top level and `outcomes[]`.
2. Keep every `Mutant` scenario with its `summary`, `diff_path`,
   `log_path`. The `Baseline` scenario is kept once as the baseline
   record.

**Test:** on every captured `outcomes.json` under `hyperray-fixtures/`,
the file parses; `caught + missed + unviable + timeout == total_mutants`;
every `Mutant.file` ends in `.rs`; every `span.start.line ≥ 1`.

**Proof:** (not built)

### Phase C — join to the manifest and to `proof.json`

**Who knows:** the mutant's `file` + `span.start.line` is the stage-1
key. `file` is relative to the tree root (measured: `noodles-bam/src/…`
on the 4-crate run).

**Steps:**

1. For each `MissedMutant`, the manifest function whose span contains
   `span.start.line` on the same `file`. Found → attach `proof_verdict`
   from `proof.json`. Not found → keep with `function_in_manifest:
   null` (a patched line outside any function the manifest saw).
2. `CaughtMutant` rows are counted, not listed.

**Test:** on every fixture, every survivor has `file` and a line; every
survivor inside a manifest function carries that function's stage-4
verdict; survivor count equals `totals.missed`.

**Proof:** (not built)

### Phase D — rerun the proof on each survivor

**Why:** `cargo mutants` runs tests. A mutant the tests miss may still
make Kani fail where it passed. That is a second, independent pin.

**Steps:**

1. For each survivor, apply its `diff_path` to a scratch copy of the
   crate (the tool wrote the diff).
2. Run stage 4 on the enclosing function only.
3. Kani `failed` → `pinned_by: "proof"`, with the failed check. Kani
   `proved` → `pinned_by: "nothing"`. Kani `blocked` → `pinned_by:
   "unknown"` with Kani's reason.

**Test:** on every fixture, every survivor has `pinned_by` in
{`proof`, `nothing`, `unknown`}; a `proof` row names ≥ 1 failed check.

**Proof:** (not built)

### Phase E — write `adequacy.json`

**Steps:**

1. Totals from the tool, plus a count of `pinned_by == "nothing"`.
2. Survivors in file-then-line order.

**Test:** valid JSON; survivor count equals `totals.missed`; every
`nothing` row's `diff_path` exists.

**Proof:** (not built)

## Order of building

B (pure reader; the scratch-crate `outcomes.json` is under
`hyperray-work/mutants-shape/`, to be copied into `hyperray-fixtures/`).
Then C, E, A. D last: it is the only phase that runs Kani again.

## What a survivor is not

- **An equivalent mutant.** go_adapter.md:901: 4 of 4 survivors in one
  run made no observable difference. A survivor is a question. Phase D
  answers part of it; stage 6 answers the rest by running original and
  mutant on the stage-3 boundary inputs; no differing input = equivalent,
  a row and never a test.
- **A test-file mutant.** `--in-diff` includes test files the patch
  touched. Rows whose `file` is under `tests/` are listed under
  `in_tests`, not `survivors`.

## Not measured

- Time for the full run on noodles v2 (185 mutants). `--list` took 1.2 s;
  the run compiles and tests each one.
- `-j` and `--shard`.

## What this stage does not do

- Does not write a test. Stage 7.
- Does not pick inputs. Stage 6, from stage-3 bounds and PICT.
- Does not read the task text. Stage 6, only for `nothing` rows.
- Does not touch the source tree; every mutation is in the tool's
  scratch copy.
