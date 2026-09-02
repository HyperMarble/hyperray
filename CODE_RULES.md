# Code rules — how every line in this repo is written

Status: PROPOSED 2026-09-02. Applies to every language in the repo.
The user checks logic by reading the plain-English explanation that
accompanies each file, so the code must be readable by someone who can
read code but does not write it daily.

## 1. Shape

- One file, one job. A file name says what it does: `extract.rs`,
  `bound.rs`. No `utils`, `helpers`, `misc`, `common`.
- No production source file over 150 lines. Split by responsibility, not
  to dodge the limit.
- No function over 40 lines. A function that needs a comment to explain
  its parts is two functions.
- Names are full words. `dst_len`, not `dl`. `prefix_len`, not `p`.
- No hidden global state. Everything a function needs comes in as an
  argument; everything it decides goes out as a return value.

## 2. Every decision is a value

- A decision that never comes back as a value cannot be tested. Arithmetic
  buried inside a loop or an `if` is moved into a named function that
  returns it. (`scratch_len` is the model: one input, one output, provable
  in 0.04 s.)
- Every bound is a named constant with the name the source uses.
  `MAX_DETECTION_PREFIX_LEN`, not `65536`. Never an invented number.
- Branches are exhaustive. In Rust, `match` with no `_` arm on enums we
  own. In Go, a `default` that returns an error, never silence.

## 3. Failure is a value too

- No panics, no `unwrap()`, no `expect()`, no `os.Exit` in library code.
  Errors are returned. Only `main` decides what to do with them.
- "Cannot do this" is always a valid answer and is always said out loud:
  `{"status":"blocked","blockers":[...]}`. Silent omission is a bug.
- A tool that is not installed, a file that does not parse, a bound that
  cannot be found — each is a named blocker with the exact reason.
- "Compiled" and "verified" are different words and are reported
  separately. Never say verified when the tool only compiled.

## 4. Nothing is measured by running the reference

- The adapter reads code. It does not execute the solution and record
  what came out. A value learned only by running is inadmissible
  (`docs/specs/evidence-rule.md`).
- Stage 3 (prove) executes the *solver*, never the program.

## 5. Tests

- Every file ships with a test in the same commit. No exceptions.
- Tests use the real fixture — noodles-296 for Rust — not an invented
  toy, wherever the real one exists.
- A test asserts a value the spec owes. A test that pins an observed
  number without a rule behind it is deleted.
- Tests run in under 60 s locally, or they are marked and run separately.

## 6. Dependencies and tools

- External tools (`charon`, `kani`, `docker`, `esbmc`) are called by
  absolute path or found once at startup, version printed, and pinned in
  the response. Never assumed present.
- No new dependency without a one-line reason in the commit message.
- Prefer the tool that already exists over code we write. We write glue
  and rules; we do not write parsers, solvers, or invariant finders.

## 7. Comments

- A comment says WHY, never WHAT. If the code needs a WHAT comment, the
  name is wrong.
- Every source file starts with one to three lines saying what it is for
  and what it must never do. That is the only header.

## 8. Delivery

- Work locally. Nothing is committed by itself.
- Each finished piece comes with a plain-English report: what it is, how
  to run it, what it changes, what was measured, what is untested.
- The user reads the report. Then commit, then push.
- Commit messages: `feat:`, `fix:`, `docs:`, `test:`, `chore:` — and the
  body names the measurement that justifies the change.

## 9. Language-specific

- **Rust**: `cargo clippy -- -D warnings` clean. No `unsafe`. Edition 2021.
- **Go**: `go vet` clean, `gofmt`. Errors wrapped with `%w`. Go 1.25+.
- **C++**: C++20, `-Wall -Wextra -Werror`, no raw `new`/`delete`, no
  exceptions across the adapter boundary.
- **Python**: type hints on every signature, `mypy --strict` clean,
  `uv` for everything, Python 3.11+.

## 10. What I never do

- Never say a tool cannot do something without having run it.
- Never shrink the user's idea to make it fit. Run it on the whole thing.
- Never edit a passing artifact.
- Never write a number in a doc that was not produced by a tool run.
