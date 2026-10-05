# hyperray

[![CI](https://github.com/HyperMarble/hyperray/actions/workflows/ci.yml/badge.svg)](https://github.com/HyperMarble/hyperray/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/HyperMarble/hyperray)](https://goreportcard.com/report/github.com/HyperMarble/hyperray)
[![Release](https://img.shields.io/github/v/release/HyperMarble/hyperray)](https://github.com/HyperMarble/hyperray/releases)
[![License](https://img.shields.io/github/license/HyperMarble/hyperray)](LICENSE)

hyperray checks whether code really does what it is supposed to do.
It works on the compiled program, the exact machine instructions that
run, and uses proofs instead of tests.

## Why

Tests only check the inputs someone thought to write down. A change can
pass every test and still be wrong. This matters more now that AI agents
write much of the code, and benchmarks judge those agents with tests.

hyperray answers with a proof: either the code meets its requirement for
every input, or here is an input where it does not.

## How it works

```
your project + a spec of what the code must do
   -> 1. build     the project, the way it builds for release
   -> 2. read      the compiled program
   -> 3. model     every instruction with the official ARM semantics
   -> 4. verify    check the spec against the code, at the ISA level
```

1. **Build.** hyperray runs the project's own build (cargo for Rust, the
   project's Python environment for Python), so it checks the exact code
   that ships, with the same versions and settings.
2. **Read.** It reads the compiled program: the original bytes, how they
   are laid out in memory, and the libraries they call.
3. **Model.** Each instruction gets its meaning from Sail, the official
   machine-readable model of the ARM processor, run by our verification
   engine and solved with Z3.
4. **Verify.** The code is checked against your spec for every input.
   Because the check runs on the processor's own instruction semantics,
   it also sees what the code really does there, including behavior the
   spec never mentioned, such as a path that can fault or write out of
   bounds.

## What you get

Every check ends in one of these answers:

| Answer | Meaning |
|---|---|
| **Proved** | The code meets the spec for every input. |
| **Disproved** | Here is an input where it does not, confirmed on the real processor. |
| **Open** | hyperray could not finish, and says where and why. This says nothing about whether the code is right. |
| **Blocked** | Something it needs is missing, such as the project's build settings. |

Each answer is tied to the exact build it came from, so a result never
outlives the code it describes.

## Status

hyperray is in early development. It does **not** yet verify a real
function end to end.

- [x] `.hray` spec format, with parser and validator
      (done, but still changing as the rest takes shape)
- [ ] Language adapters that build a whole Rust or Python project
- [ ] Verification engine
- [ ] Verification: prove code meets its spec, at the ISA level
- [ ] Git-style record tying every proof to the exact build it covers
- [ ] Judge for coding benchmarks, and a training signal
- [ ] x86 support

Since hyperray is in early development, expect a lot of changes across
the codebase, even in parts that are already done. The latest release,
v0.1.2, is an earlier design.

## Repository map

| Folder | What it holds |
|---|---|
| `language/` | one adapter per language, which builds the project |
| `loader/` | reads compiled programs and decodes instructions (Rust) |
| `contract/` | the `.hray` spec format: parser and validator (Rust) |
| `solver/` | reads the solver's answers (Rust) |
| `rechecker/` | confirms counterexamples on the real processor (Rust) |
| `machine/` | the old Go engine connection, to be replaced |
| `cmd/hyperray/` | the command-line tool (Go) |
| `skills/` | instructions for AI agents writing specs |
| `docs/` | how the parts work |

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for
setup, tests, and how we work.

## License

[MIT](LICENSE)
