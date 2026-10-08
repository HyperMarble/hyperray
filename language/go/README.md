# Go adapter

Builds a Go module the module's own way, records every file the build made
and everything that made it that way, and hands the files to the loader.

It never looks inside the code. The compiler turns every Go shape into
machine code; the adapter picks that machine code up. So "does the adapter
handle generics?" means "does the compiler build generics into a program,
and does the adapter hand that program over?"

It never refuses a build for the way it is built, either. Every mode Go
documents is handled or written into the record by hash. The only
refusals left are Go's own, repeated word for word, and a file the adapter
needs and cannot read.

## Layout

```
adapter.go, assemble.go, types.go   the door in, and the record's assembly
record/    the record's shape, hashing, the refusal reasons, the build choice
tool/      running the go tool, the toolchain, go env, the C toolchain, the flags
module/    the module, its packages and sources, nested, local and excluded ones
keep/      the keep list and the overlay that adds it
build/     building, reading Go's stamp back, naming what came out
tests/     end-to-end tests, each on a real module written for it
```

## Every function gets machine code

Go only makes machine code when it links a program, and two of Go's own
rules can leave a function without a body of its own: the linker drops
what nothing reaches, and the compiler folds small functions into their
callers. So for every package the adapter builds its test program (what
`go test` builds, which links the package's real code) and adds, through
Go's `-overlay` so the module's folder is never touched, a generated test
file that refers to every function and method by value: the package's own
files, its cgo files, its own test files, and, in a second generated file,
its external `_test` package. That keeps each one linked with its own
machine code. Programs (main packages) get their executable as well, named
by the mode Go built them as: program, shared library, plugin, C archive.

Generic functions are the one thing Go cannot refer to by value: Go makes
one copy per concrete type the project uses, each a function of its own,
and those copies are kept through the code that uses them. A generic
function nobody uses has no machine code, by Go's rule.

## Coverage of Go shapes

Proof: Go's own test suite (`$GOROOT/test`, Go 1.26.0), every test that
builds into a package on an ARM Mac, one module per test: 1,677
single-file tests, 258 folder tests, and the 22 programs printed by the
suite's two-stage tests, 1,957 in all.

Every program Go built, the adapter handed over: 1,893 of 1,893.

Counts per folder of Go's suite are "handed over / Go built":

| Folder | Handed over / Go built |
|---|---|
| `fixedbugs/`, every shape a fixed compiler bug touched | 1,233 / 1,233 |
| `typeparam/`, generics | 271 / 271 |
| top level: functions, closures, defer, maps, slices, numbers, `for`, `if`, … | 191 / 191 |
| `codegen/` | 81 / 81 |
| `ken/` | 40 / 40 |
| `abi/`, calling convention | 38 / 38 |
| `chan/`, channels and select | 19 / 19 |
| `interface/` | 15 / 15 |
| `dwarf/`, `simd/`, `arenas/` | 5 / 5 |

Among the 1,893 are 22 records with zero built files, tests whose every
file carries a build line for another chip or system (`amd64`, `js`,
`aix`, …): `go build ./...` builds nothing for them and exits clean, and
so does the adapter, listing each left-out folder with Go's own reason and
its files by hash. And 2 records whose test program is listed as not
built: their own flag (`-ldflags -strictdups=2`,
`-gcflags=-d=maymorestack=main.f`) links the program but forbids linking
a test program, and the record says so in Go's words.

The other 64 Go itself refused to build, so there was nothing to hand over:

- 56 are compiler tests by their header, `compile` or `compiledir`: Go's
  runner compiles them and stops, and `go build` has no program to make.
  One of them is also outside module mode, an import path with `Þ`, which
  the modules reference restricts to ASCII.
- 6 declare a function with no body and no assembly file, on purpose: Go's
  runner calls the compiler without `-complete`, which `go build` always
  adds when a package has no assembly (`cmd/go/internal/work/gc.go`).
- 2 have their bodies in assembly for another chip, x86 and wasm; both
  cross-build for that chip.

The run: the lab's wrapper turns each test into a module with the flags
from its header, builds it once with the go tool alone and once through
the adapter, and writes one line per test. The lab lives beside the repo
on the maintainers' disk, not in it yet.

## Coverage of Go's build modes

The second owner's list is `go help`. Each list below is read by a test
that fails by name when a future Go adds an entry the adapter does not
place:

| List | Entries | Test |
|---|---|---|
| `go help build`, every flag | 32, each recorded, six handled | `tool/flags_test.go` |
| `go help list`, every `…Files` field | 19, 16 hashed, 3 named as not inputs | `module/sources_test.go` |
| `go help environment`, every tool | 5, `CC CXX FC AR PKG_CONFIG`, each hashed when present | `tool/ctools_test.go` |
| `go help buildmode`, every mode | 8, each with a label | `build/kind_test.go` |

The six handled flags pull something into the build from outside the
project, and that something is hashed: `-overlay` (merged with the keep
overlay, every backing file), `-modfile` (the alternate go.mod and its
sum), `-mod=mod` (pin files before and after), `-toolexec` (the wrapper
program), `-C` (that folder becomes the root), and from the environment
`GOCACHEPROG` (the cache program).

## What the record holds

- every file the build made: which package, what kind, its hash, every
  source file compiled into it by hash, the PGO profile Go applied by hash,
  and Go's own stamp read back out of the file: Go version, every module
  with version and checksum, every setting Go stamps
- the go installation: version, host, experiments, `bin/go`, every tool in
  `GOTOOLDIR`, and one hash over all of `GOROOT/src`
- what was asked, and what the build ran with: the tags Go stamped, every
  flag in the order Go read them, `go.mod`, `go.sum` and the active
  `go.work` by hash, and after the build the pin files again when they
  moved
- every `go env` setting, sorted, secrets hidden
- the C toolchain when cgo is on: the C compiler and SDK, C++, Fortran,
  archiver, pkg-config, and the linker the C compiler reports, each by hash
- every package with native code: cgo, C, C++, Objective-C, headers,
  Fortran, assembly, SWIG, prebuilt objects
- nested modules, each with its own record; modules under folders Go's
  pattern rules skip, by hash; local modules from a directory replace or a
  workspace, their compiled sources by hash; folders Go excluded, with its
  reason and their files by hash; test programs the project's flags forbid,
  with Go's words
- the OS build, by exact version, so a record names the kernel it ran
  under

## Not added yet

- The kernel's code behind each system call: the record pins the OS
  build, the engine takes the call's return as a written contract, and the
  kernel's own code for that call is not walked yet. The list to cover is
  the kernel's call table, and a program reaches a handful of it.
- The two lab limits in the coverage run are gone; the lab itself is not
  in the repo.
