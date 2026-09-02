# Rust adapter — build plan

Status: PROPOSED — 2026-09-02. Code starts after the user accepts.

## What is wrong today

`internal/frontend/rust/` reads Rust with its own lexer and parser written
in Go (`lexer.go` 247 lines, `parser.go` 791). It accepts a "closed subset":
no method receivers (`parser.go:165`), most expressions rejected
(`parser.go:459`). It cannot read noodles-296 — async, generics, trait
objects, 14 files. So the spec rows are written by a human, and yesterday
the human wrote 38 rows when the true count was 107.

Everything downstream of the parser is sound and stays:
`semanticir` (the four equalities), `proof` (Z3 + enumeration),
`mutate`, `enforce`, `certificate`.

## The change, in one line

**Stop parsing Rust in Go. Ask the Rust compiler, and let a Rust program
do the Rust-specific work.** The Go CLI orchestrates; the Rust adapter
extracts, bounds, and proves.

## Layout

```
hyperray/
  cmd/hyperray/            Go CLI, stays
  internal/...             stays; frontend/rust/{lexer,parser}.go retired
  adapters/rust/           NEW — a Cargo crate, binary `hyperray-rust`
    src/main.rs            stdin JSON request -> stdout JSON response
    src/extract.rs         stage 1
    src/bound.rs           stage 2
    src/prove.rs           stage 3
    src/adequacy.rs        stage 4
  docs/rust_adapter.md     the measurements this plan rests on
```

The Go side talks to the adapter over the protocol that already existed:
one JSON request on stdin, one JSON response on stdout, and
`{"status":"blocked","blockers":[...]}` when it cannot finish. Nothing
silently omitted.

## Stage by stage — what the adapter calls, what it returns

| stage | calls | returns to the CLI |
|---|---|---|
| 1 EXTRACT | `charon` (install first; fallback rustdoc JSON + `-Zdump-mir`) | manifest: every public fn, its typed params, finite domain per type, enum arms, named constants, branch sites, compiler-inserted overflow checks |
| 2 BOUND | LoopInvGen (`docker padhi/loopinvgen`) with a SyGuS file the adapter writes from the MIR loop | one invariant per loop, or `blocked` — `false` and guard-echo are rejected before return |
| 3 PROVE | `cargo kani autoharness` + contracts | per function: PASS with check count, or FAIL with the exact counterexample |
| 4 ADEQUACY | mutate source, rerun stage 3 | list of surviving mutants = candidate missing rows |

Stages 5–6 stay in the Go CLI, once, shared by all languages. Emit design
is owed by the user and not started here.

## Rules the adapter enforces (from rust_adapter.md §12)

- Every bound cites a constant in the patch. Invented bounds are refused.
- A `false` invariant or a guard-echo invariant is a translation bug, not a result.
- Compiling is not verifying. Both reported separately.
- A row derived only by running the reference is inadmissible (evidence rule).
- `blocked` is always a valid answer; a silent pass never is.

## Order of work

1. `adapters/rust` crate skeleton, protocol I/O, `--version`. Build.
2. Install Charon. Stage 1 on noodles-296. Show the manifest.
3. Stage 3 via Kani on the same manifest. Show the 322 defect found by the adapter, not by hand.
4. Stage 2 on both loops. Show the two invariants.
5. Stage 4. Show at least one surviving mutant.
6. Wire the Go CLI to call it. One command, end to end, on noodles-296.
7. Report in plain English. User reads. Commit.

Each step ends with real output pasted, not a summary.

## What this does not do

- Does not touch `spec.md` format or the spec skill.
- Does not build plan or emit.
- Does not claim any other language.
