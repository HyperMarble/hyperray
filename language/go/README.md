# Go adapter

Builds a Go module the module's own way, records every file the build made
and what made it that way, and hands the files to the loader.

It never looks inside the code. The compiler turns every Go shape into
machine code; the adapter picks that machine code up. So "does the adapter
handle generics?" means "does the compiler build generics into a program,
and does the adapter hand that program over?"

## Every function gets machine code

Go only makes machine code when it links a program, and two of Go's own
rules can leave a function without a body of its own: the linker drops
what nothing reaches, and the compiler folds small functions into their
callers. So for every package the adapter builds its test program (what
`go test` builds, which links the package's real code) and adds, through
Go's `-overlay` so the module's folder is never touched, one generated
test file that refers to every function and method in the package by
value. That keeps each one linked with its own machine code. Programs
(main packages) get their executable as well.

Generic functions are the one thing Go cannot refer to by value: they have
machine code only where they are used with concrete types, and those
copies are kept through their callers.

## Coverage of Go shapes

Proof: Go's own test suite (`$GOROOT/test`, Go 1.26.0), every test that
builds into a package on an ARM Mac: 1,677 single-file tests and 258
folder tests, 1,935 in all.

Every program Go built, the adapter handed over: 1,842 of 1,842, with two
exceptions explained below.

Counts per shape are "handed over / Go built".

- [x] Generics (type parameters): 271 / 271
- [x] Interfaces, methods, embedding: 30 / 30
- [x] Functions, closures, defer, recover, panics: 48 / 48
- [x] Channels, goroutines, select: 30 / 30
- [x] Structs, arrays, slices, maps, strings: 36 / 36
- [x] Numbers, constants, conversions, iota: 85 / 85
- [x] Calling convention (ABI), assembly, code generation: 117 / 117
- [x] Fixed compiler bugs, every shape (`fixedbugs/`): 1,212 / 1,212
- [x] Other language tests (`for`, `if`, `nil`, `named`, ...): 13 / 13

The other 93 never reached the adapter:

- 68 Go refused: 38 compile-only tests that are `package main` without a
  `main` function (Go will not make a program of those), 27 whose bodies
  live in assembly for other chips, 3 stopped by Go's own linker rules
  (`linkname`, x86 instructions, a missing assembly symbol).
- 22 are for other chips or systems, by their own `//go:build` line
  (`amd64`, `js`, `aix`, `gccgo`, experiments).
- 2 pass a flag of their own that forbids a test program (`-strictdups=2`,
  `-d=maymorestack=main.f`): the adapter built their executables, then
  reported blocked because the second program could not be made.
- 1 could not be set up by the lab's wrapper (a `Þ` in an import path,
  which a module path cannot hold). Lab work, not the adapter.

## What the record holds

- every file the build made: which package, program or test program, its
  hash, and Go's own record read back out of the file: Go version, every
  module version with its hash, every build setting (tags, gcflags,
  ldflags, race, cgo flags, GOARCH, GOOS, GOARM64), the git commit
- the go tool: version, host, hash
- what was asked: build tags and extra flags; `go.mod`, `go.sum` and any
  workspace file, by hash
- every `go env` setting, sorted, secrets hidden
- packages that compile C, C++ or assembly, and the C compiler and SDK
  when cgo is on
- the OS build
