# ray

ray verifies any task with a bounded scope. The statement, the solution, and
the tests are checked against each other over the task's whole finite
behavior space — a green run means they agree, provably. Anything unverified
is reported, never assumed.

Currently: coding tasks in Python, Rust, and C++.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/HyperMarble/ray/main/install.sh | sh
```

```sh
brew install hypermarble/tap/ray
```

```sh
go install github.com/HyperMarble/ray/cmd/ray@latest
```

## Use

```sh
ray init my-task     # spec template + schema reference
# author spec.md
ray verify my-task   # the whole ladder, one exit code
```

`ray help` lists every command; `ray update` fetches the latest release.

## Beyond benchmarks

1. PR-scoped verification
2. Runtime reliability layer (Anchor)
3. Open-ended scope
