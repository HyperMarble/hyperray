# CLAUDE.md - hyperray

> Hyperray v0.10 scope is Python/Rust/C++ bounded task/PR verification only.
> The only architecture sources of truth are
> `docs/specs/finalarchitecture.md` and `docs/specs/whole flow.md`, verified by
> `docs/specs/architecture-freeze.sha256`. The dated v0.1 design documents are
> historical and must not drive implementation.

## Project Overview

hyperray verifies that an agent's (AI or human's) code logic is actually
correct, using formal methods layered with testing, not testing alone.
It sits upstream of eval-environment frameworks like Harbor and
Inspect: hyperray checks whether a task's spec, tests, and solution are
internally sound before any agent or eval framework touches it.

- **spec-lint**: checks `spec.md`'s condition tables for completeness
  and disjointness — every parameter combination has exactly one row,
  no two rows conflict
- **coverage**: wraps PEV to generate the combinatorial test matrix
  from `spec.md`'s parameters, cross-checks it against tests/ for gaps
- **oracle**: proves a simplified reference model against `spec.md`
  (touchstone-prover for Python, Verus for Rust, ESBMC for C++)
- **diff-test**: runs the real solution against the proven oracle on
  generated inputs, reports disagreement
- **dep-harvest**: harvests a pinned dependency's own edge-case test
  values into coverage's value lists, cached per (dependency, version)

## Quick Start Commands

```bash
# Build the CLI
go build ./...

# Check a task's spec.md
go run ./cmd/hyperray spec-lint examples/fhplex-task/spec.md

# Run the test suite
go test ./...
```

## Repository Structure

```
hyperray/
├── cmd/hyperray/               # CLI entrypoint (cobra)
│   ├── main.go            # root command, subcommand registration
│   └── speclint.go        # `hyperray spec-lint` subcommand
├── internal/
│   └── specparser/        # spec.md condition-table parser
│       └── specparser.go  # Parse() -> []Table
├── tests/                 # external test package, tests/*_test.go
│   └── specparser_test.go
├── skills/
│   └── spec/
│       └── SKILL.md       # guides writing spec.md for an existing task
├── examples/
│   └── fhplex-task/
│       └── spec.md        # worked example, real Shipd/Olympus task
├── docs/
│   └── specs/              # design docs (source of truth for scope/architecture)
└── go.mod
```

## Key Concepts

### Task bundle

A hyperray task is a directory shaped like this:

```
task/
  instruction.md   # prose, for a human or an agent to read as the ticket
  spec.md          # condition tables — the only file hyperray's layers read
  hyperray.toml         # verifier timeouts, target language
  environment/     # Dockerfile
  tests/
  solution/
```

The authoring skill reads the instruction/issue, base code, reference diff, and
relevant environment to create `spec.md` before tests are used as enforcement
evidence. Production proof stages consume compiled Spec Semantic IR, not prose.

### Result

Every stage reports structured evidence, but only the exact Semantic-IR proofs
can produce the final `VERIFIED` verdict:

```go
type Result struct {
    Pass        bool
    Explanation string          // which clause/combination/line, and why
    Metadata    map[string]any  // structured detail for machine consumption
}
```

## Development Setup

```bash
git clone https://github.com/HyperMarble/hyperray.git
cd hyperray

go mod tidy
go build ./...
go test ./...
```

## Testing

- Tests live in the top-level `tests/` package (`tests/*_test.go`), not
  co-located `_test.go` files inside `internal/` — they import internal
  packages the same way any real consumer would.
- Prefer a real fixture over a synthetic one where one exists — e.g.
  `tests/specparser_test.go` parses the actual
  `examples/fhplex-task/spec.md`, not just an inline string, so a
  regression in that file's real Markdown shape is what catches a
  parser bug, not a hand-picked fixture that happens to avoid it.

```bash
go test ./...
go vet ./...
```

## Code Style and Linting

- No comments unless the WHY is non-obvious. Never restate what the
  code already says.
- Commit messages use conventional prefixes: `feat:`, `fix:`, `docs:`,
  `chore:`, `test:`.

## Key Patterns

### Cobra subcommands

Each layer gets its own file under `cmd/hyperray/`, registered once in
`main.go`:

```go
// cmd/hyperray/speclint.go
func newSpecLintCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "spec-lint <spec.md>",
        Short: "Check a spec.md's condition tables for completeness and disjointness",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // parsing/logic lives in internal/, not here
        },
    }
}

// cmd/hyperray/main.go
root.AddCommand(newSpecLintCmd())
```

### Parser output feeds the checker, never the terminal, directly

`internal/specparser.Parse` returns structured `[]Table`; the CLI layer
is only responsible for turning a `Result` into terminal output and a
`logs/<layer>/` entry — it never formats parser output itself.

## Common Tasks for AI Assistants

### Writing spec.md for a task

Use the `spec` skill (`skills/spec/SKILL.md`). Don't derive `spec.md`
from `instruction.md` in one pass — clause by clause, `spec-lint` in
the loop, resolving each ambiguity into the table as it's found.

### Adding a new layer (coverage, oracle, diff-test, dep-harvest)

1. Add `cmd/hyperray/<layer>.go` with a cobra subcommand
2. Wire it into `root.AddCommand` in `cmd/hyperray/main.go`
3. Put parsing/logic in its own `internal/<layer>/` package, not in
   `cmd/hyperray` directly
4. Return the shared `Result{Pass, Explanation, Metadata}` shape

## File Naming Conventions

- Go files: standard lowercase, no underscores (`speclint.go`)
- Test files: `tests/<package>_test.go`
- Config: `hyperray.toml`
- Markdown: `instruction.md`, `spec.md`, `SKILL.md`

## Important Notes

- Go 1.25+ required
- `spec.md` is the production artifact hyperray's tools consume at
  runtime — never write process/decision narrative into it; that
  belongs in `skills/spec/SKILL.md`
- v0.1.0 scope: Python, Rust, C++ solution targets only; Go deferred —
  no oracle-layer tool exists yet
- `hyperray check` retains spec-lint, coverage/PICT, oracle, diff-test, and
  dep-harvest, then performs the mandatory exact reference,
  false-positive, false-negative, and reference-acceptance checks from the
  frozen architecture. Diagnostic passes cannot manufacture `VERIFIED`.
