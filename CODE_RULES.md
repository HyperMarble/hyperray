# Code rules for this repo

Status: ACCEPTED 2026-09-02. Applies to every language in the repo.

The reasoning rules are Linus Torvalds's kernel coding style. The full
text with notes is in the workspace `AGENTS.md`. This file holds only what
is specific to Hyperray. Where the two disagree, `AGENTS.md` wins.

## 1. Shape

- One file, one stage. The file name is the stage: `extract.rs`,
  `shape.rs`, `bound.rs`, `prove.rs`, `adequacy.rs`, `emit.rs`. No
  `utils`, `helpers`, `misc`, `common`.
- No production file over 75 lines. If a stage needs more, it becomes a
  folder with the same name.
- No function over 40 lines. No nesting deeper than 3 levels. When either
  limit is hit, split the function. Never re-indent to dodge it.
- Same file names in all four adapters. `shape.rs`, `shape.cpp`,
  `shape.go`, `shape.py` do the same job.
- Names are full words. `prefix_len`, not `p`. A name says the job, so the
  body needs no comment.

## 2. Every decision is a value

- A decision that never returns cannot be tested or proved. Arithmetic
  inside a loop or an `if` moves into a named function that returns it.
- Every bound is a named constant with the name the source uses.
  `MAX_DETECTION_PREFIX_LEN`, never `65536`. No invented numbers.
- Branches are exhaustive. Rust: `match` with no `_` arm on enums we own.
  Go: a `default` that returns an error, never silence.

## 3. Failure is a value

- No panics, `unwrap`, `expect`, `let _ =`, or `os.Exit` in library code.
  Errors return with their cause. Only `main` decides what to do.
- "Cannot do this" is a valid answer and is always said:
  `{"status":"blocked","blockers":[...]}`. Silent skip is a bug.
- "Compiled" and "verified" are different words, reported separately.

## 4. Evidence

- The adapter reads code. It never runs the solution and records what
  came out. A value learned only by running is inadmissible
  (`docs/specs/evidence-rule.md`).
- The prove stage runs the solver, never the program.

## 5. Comments

Linus's rule, word for word: "NEVER try to explain HOW your code works in
a comment: it's much better to write the code so that the working is
obvious." And: "you want your comments to tell WHAT your code does, not
HOW."

- A comment inside a function is permitted when it adds information that
  the code cannot express clearly. It states WHAT or WHY, never HOW.
- A head comment only when the name is not enough. It states purpose,
  inputs, and return. Never the steps.
- A reason where the code cannot carry it: a number, an order that
  matters, a lock or a retry whose cause is not visible.
- Every source file opens with one to three lines: what it is for, and
  what it must never do. That is the only file header.

## 6. Tests

- Every file ships with a test in the same commit.
- Tests run on real fixtures under `fixtures/`. The adapter never knows a
  fixture name. Code that passes one fixture is a script. Code that passes
  all of them is the adapter.
- A test asserts a value the code declares. A test that pins an observed
  number with no rule behind it is deleted.

## 7. Tools

- External tools (`charon`, `kani`, `loopinvgen`, `esbmc`) are found once
  at startup, version printed, and pinned in the response. Never assumed.
- Prefer the tool that exists over code we write. We write glue and rules,
  not parsers, solvers, or invariant finders.
- No new dependency without a one-line reason in the commit body.

## 8. Commits

- One logical change per commit. Push at once.
- Subject: `type(scope): imperative summary`, 75 characters max.
  Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`.
- Blank line, then the body at 75 columns. The body says the problem, why
  this change, and the number that was measured. Never what the diff does.
- No `Co-authored-by`.

## 9. Per language

Same rules everywhere. Each language keeps its own formatter and checker,
warnings as errors.

- **Rust**: `rustfmt`, `clippy -- -D warnings`. No `unsafe`.
- **Go**: `gofmt`, `go vet`. Errors wrapped with `%w`.
- **C++**: `clang-format`, `clang-tidy`, `-Wall -Wextra -Werror`. No raw
  `new`/`delete`. No exceptions across the adapter boundary.
- **Python**: `ruff format`, `ruff check`, `mypy --strict`. `uv` for all.

## 10. Never

- Never say a tool cannot do something without having run it.
- Never shrink the task to fit. Run the whole patch.
- Never edit a passing artifact.
- Never write a number in a doc that a tool did not produce.
