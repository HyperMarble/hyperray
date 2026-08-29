# hyperray

[![CI](https://github.com/HyperMarble/hyperray/actions/workflows/ci.yml/badge.svg)](https://github.com/HyperMarble/hyperray/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/HyperMarble/hyperray)](https://goreportcard.com/report/github.com/HyperMarble/hyperray)
[![Release](https://img.shields.io/github/v/release/HyperMarble/hyperray)](https://github.com/HyperMarble/hyperray/releases)
[![License](https://img.shields.io/github/license/HyperMarble/hyperray)](LICENSE)

hyperray is an open source verification engine for tasks with a bounded scope. It proves that the three things an author writes — the statement, the solution, and the tests — agree with each other over the task's whole finite behavior space. A green run means they agree, provably; anything hyperray cannot verify is reported as untried, never assumed. Currently hyperray verifies coding tasks in Python, Rust, and C++.

hyperray sits upstream of evaluation pipelines: before any agent or grader touches a task, hyperray answers whether the task itself is sound — no incorrect solution can pass its tests, and no correct solution is rejected by them.

## Features

* **Fail-closed ladder**: `hyperray verify` chains every gate into one command with one exit code. A rung that cannot run reports blocked instead of silently passing.
* **Statement gate**: the problem statement is checked like a reviewer reads it — ASCII only, word budget, and every promise-bearing sentence must have spec rows behind it.
* **Repository lint gate**: the host repository's own configured linters (ruff, cargo clippy, clang-format/tidy) run against the solution files. No config in the repo means no gate — hyperray never invents style rules.
* **Fail-to-pass**: every test must fail on the clean base and pass with the solution. A test that passes without the solution enforces nothing.
* **Test hygiene**: the suite runs twice and in reverse order; the same tree must give the same verdict, or the verifier cannot be trusted as an instrument.
* **Per-rule enforcement**: for each spec rule hyperray constructs the wrong solutions that violate it (wrong message, wrong type, no error) and demands the tests kill every one. Survivors are named holes, never shrugs.
* **Generated probes**: hyperray watches the real solution behave — one probe per rule, generated in the task's language; the author writes a single entry per operation.
* **Bundle completeness**: the test patch must deliver the canonical runner; a patch that drops it fails locally in milliseconds instead of remotely in minutes.
* **Multi-language**: pytest, cargo, ctest, and googletest binaries all speak through one runner abstraction.
* **Self-updating binary**: `hyperray update` fetches the latest release and swaps itself atomically.

## Architecture

The frozen design lives in [docs/specs/finalarchitecture.md](docs/specs/finalarchitecture.md) and [docs/specs/whole flow.md](docs/specs/whole%20flow.md), digest-guarded by [docs/specs/architecture-freeze.sha256](docs/specs/architecture-freeze.sha256). The evidence rule and proof requirements are in [docs/specs](docs/specs).

## Install & Run

**Install the release binary** (macOS and Linux, arm64 and amd64):

via Homebrew:

```sh
brew install hypermarble/tap/hyperray
```

via Curl:

```sh
curl -fsSL https://raw.githubusercontent.com/HyperMarble/hyperray/main/install.sh | sh
```

or from source (Go 1.25+)

```sh
go install github.com/HyperMarble/hyperray/cmd/hyperray@latest
```

**Verify a task:**

```sh
hyperray init my-task     # writes a spec template and the schema reference
# author spec.md
hyperray verify my-task   # the whole ladder, one exit code
```

Release artifacts ship with a `checksums.txt`; verify a download with `shasum -a 256 -c checksums.txt`.

## Commands

| Command | Purpose |
|---|---|
| `hyperray init` | Start a task: spec template plus schema reference |
| `hyperray lint` | Compile spec.md strictly against the frozen instruction |
| `hyperray verify` | Run the whole authoring ladder |
| `hyperray check` / `hyperray start` | Verify a finished frozen task end to end |
| `hyperray harvest` | Mine a pinned dependency's own tests for edge values |
| `hyperray version` / `hyperray update` | Print the build; self-update to the latest release |

Internal rungs (`prose-lint`, `repo-lint`, `bridges-gen`, `enforce`, `hygiene`, `rows`) are driven by `verify` and callable directly for debugging.

## Compatibility

| Language | Test frameworks | Lint gates |
|---|---|---|
| Python | pytest | ruff |
| Rust | cargo test | cargo clippy |
| C++ | ctest, googletest binaries | clang-format, clang-tidy |

## Roadmap

1. PR-scoped verification
2. Runtime reliability layer (Anchor)
3. Open-ended scope

## Changelog

One-line entries per release in [CHANGELOG.md](CHANGELOG.md).

## License

hyperray is available under the [MIT license](LICENSE).
