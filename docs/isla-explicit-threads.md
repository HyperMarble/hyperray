# Explicit Isla initial threads

This leaf connects a finite, ordered set of initial machine threads to one
loaded ELF through the existing axiomatic Isla backend. It does not model Rust
thread creation, scheduling, fairness, or full language coverage.

`ProgramBoundary.Threads` accepts a nonempty `[]ThreadEntry`. Each entry names
an instruction-start address and its own concrete `InitialRegisters` slice.
Thread order becomes the contiguous native thread IDs. Legacy
`ThreadAddress` and `InitialRegisters` remain the one-thread API; mixing them
with `Threads` is rejected.

The builder checks every explicit address against the loader's decoded
instruction inventory, sorts copied register assignments independently, emits
each loaded section once, and writes one `[thread.N]` table per declared
thread. For two-digit thread counts it zero-pads table names so native lexical
TOML iteration preserves caller order; one- and two-thread output remains
unchanged. Repeated entries are valid. A sequential memory profile rejects
more than one initial thread before any native tool runs; axiomatic memory
remains the multi-thread route.

The pinned native formatter assigns displayed register IDs with
`litmus.threads.iter().enumerate()` (`isla-axiomatic/src/litmus/format.rs:265`),
while TOML thread tables are read through the lexical map iterator
(`isla-axiomatic/src/litmus.rs:1088-1100`). The padding therefore protects the
declared numeric identity rather than relying on table spelling.

`ProgramEvidence.ThreadEntries` is an ordered opaque identity string. Semantic
reports retain `SemanticThread` records, including each thread's ID, entry
address, and instruction inventory. The executable join requires the reported
count, IDs, own entry events, and aggregate instruction set to match the
declared query. A missing or extra thread therefore returns an engine error and
no verdict.

## Earlier recorded evidence

The following results predate this resumed session. The referenced logs are
absent from the current filesystem. This session did not reproduce the native
results. The current status is in the next section.


The focused package tests, full Go suite, and `go vet ./...` pass with one
shared Go temp/cache directory. The real execution-guards run compiles
`branch.rs` and `atomic_shared.rs` with rustc 1.98.0 and LLD 22.1.8. The
strengthened two-thread acceptance, the 11-thread lexical-order acceptance,
and the AtomicU64 shared-memory acceptance pass together in 93.529s. The
11-thread query and footprint stage each use one worker while the independent
PC-visit bound remains 2. The AtomicU64 proof restricts the reader to `{0,1}`;
separate queries reach 0 and 1, and its trace partitions show a `write-mem` in
writer thread 0 and a `read-mem` in reader thread 1 at the same loaded ELF
address (`0x80101000`). Exact logs:

- `/private/tmp/hyperray-explicit-threads-go-test-machine-final.log`
- `/private/tmp/hyperray-explicit-threads-go-test-all-final2.log`
- `/private/tmp/hyperray-explicit-threads-go-vet-final2.log`
- `/private/tmp/hyperray-explicit-threads-real-execution-guards.log`
- `/private/tmp/hyperray-explicit-shared-atomic-real.log`
- `/private/tmp/hyperray-explicit-threads-real-three-final.log`

This evidence remains limited to explicit Isla initial-thread integration; it
does not establish Rust spawn support, a scheduler contract, fairness, or full
Rust coverage.

## Historical resumed measurements: 2026-09-06, 07:46 UTC

The resumed session preserved the production implementation and native tests.
It repaired the public negative tests to use accepted baselines and exact
errors. The parser-to-join test now uses valid instruction encodings at both
addresses. It accepts an equal-entry baseline, rejects both sibling orders
with the entry error, and accepts a shared instruction prefix.

The following commands returned exit status 0:

```sh
go test ./machine/isla -count=1
go test ./... -count=1
go vet ./...
```

Logs use the prefix `/private/tmp/hyperray-explicit-resume-`:

- `focused-before.log`: Original three focused tests passed.
- `focused-compile-error.log`: The first repair used the nonexistent `Error.Kind` field. Compilation failed. The repair now uses `Error.Code`.
- `focused-after.log`: Six focused tests and their negative cases passed.
- `machine.log`: Package tests passed in 8.725 seconds.
- `all.log`: All Go packages passed. The Isla package took 9.325 seconds.
- `vet.log`: Vet returned no diagnostics.
- `status.log`: The package, full-suite, and vet commands and exit statuses.

The native acceptance command returned exit status 1:

```sh
go test -tags=isla_integration ./machine/isla -run '^(TestRealRustExplicitThreads|TestRealRustExplicitThreadOrdering|TestRealRustExplicitSharedAtomic|TestThreadTraceMemoryOwnershipRejectsConcentratedEvents)$' -count=1 -v
```

`real-three.log` records three infrastructure errors, no skipped cases, and a
pass for the concentrated-memory-events negative test. The compiler and linker
completed. Each native test then stopped with `tool_not_found` for
`/private/tmp/hyperray-execution-guards.JlGvKA/target/release/isla-axiomatic`.
The other two supplied native binary paths are also absent.

The single-thread regression command also returned exit status 1:

```sh
go test -tags=isla_integration ./machine/isla -run '^(TestRealRustExecutablePrograms|TestRealRustSequentialMixedWidth|TestRealRustTypedTrapState)$' -count=1 -v
```

`single-thread.log` records the same missing-tool error for the three selected
tests, including all three executable-program subtests. No case was skipped.
These errors do not establish a semantic regression or a native acceptance pass.

Both commands used the supplied `HYPERRAY_*` paths and shared Go directories:
`GOTMPDIR=/private/tmp/hyperray-go-shared.JFtnZG`,
`GOCACHE=/private/tmp/hyperray-cache-shared.BBLSij`, and `TMPDIR=/private/tmp`.
`GOPROXY=off` and `GOTOOLCHAIN=local` prevented dependency downloads.
The measured compiler was rustc 1.98.0. The measured linker was LLD 22.1.8.
Other native binaries under `hyperray-research` had different digests from
`tools/isla/measured-execution-guards.json`. This session did not substitute them.

That historical native acceptance attempt remains blocked because the pinned
binaries were absent. Coordinator review remained pending at that point. The
current rebuilt acceptance is recorded below. No commit or push occurred.

After the final variable-name cleanup, `go test ./... -count=1` and
`go vet ./...` again returned exit status 0. Their logs are
`/private/tmp/hyperray-explicit-resume-final-all.log` and
`/private/tmp/hyperray-explicit-resume-final-vet.log`. The Isla package took
6.598 seconds. `gofmt -l` returned no changed-file names. `git diff --check`
returned no diagnostics. The two new test files contain 69 and 44 lines.

The focused command for `focused-after.log` was:

```sh
go test ./machine/isla -run 'TestProgramSemanticsRejectsThreadRecordCountAndAggregateMismatch|TestProgramSemanticsRequiresEachThreadOwnEntry|TestSemanticParserJoin|TestBuildProgramRejectsExplicitThreadInputs|TestBuildProgramRejectsUnsupportedTypedStateForMultipleThreads' -count=1 -v
```

## Current rebuilt acceptance: 2026-09-06, 08:17 UTC

The missing pinned tools were rebuilt in a new internal-disk scratch directory
from source commit `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6` with
`execution-guards-v1.patch` (SHA-256
`fd5c8b12d4cb45b1405ed3f8a814c5de06eb39eb5c7aad11938d5eec10bf6af8`). The
locked offline release build used `CARGO_BUILD_JOBS=2` and produced the three
required tools. Build task `025373gj62` returned exit status 0.

```sh
CARGO_BUILD_JOBS=2 cargo build --locked --offline --release \
  --bin isla-axiomatic --bin isla-litmus-dump --bin isla-footprint
```

The rebuilt tool version is
`v0.2.0/z3-5.1.0.0/candidate-status-v1/executable-entry-v1/initialized-memory-v1/sequential-memory-v1/typed-initial-state-v1/forbidden-model-calls-v1/register-fields-v1/execution-guards-v1`.
The measured SHA-256 identities are:

- `isla-axiomatic`: `8da83833f3d2a32b67f6bcbf31e46934757077172580d517e414265a496effaf`
- `isla-litmus-dump`: `012648a9b3288e5d297bf83b5735ed3af9eaa744c9d538c340dc8eb053e11deb`
- `isla-footprint`: `c736e69bfc092b262270c20f5e148760a4e11b7ac4dca1ab14a1adfe62800d58`

Using those paths, test task `318273o65j` returned exit status 0 for both
commands:

```sh
go test -tags=isla_integration ./machine/isla \
  -run '^(TestRealRustExplicitThreads|TestRealRustExplicitThreadOrdering|TestRealRustExplicitSharedAtomic)$' \
  -count=1 -v

go test -tags=isla_integration ./machine/isla \
  -run '^(TestRealRustExecutablePrograms|TestRealRustSequentialMixedWidth|TestRealRustTypedTrapState)$' \
  -count=1 -v
```

The explicit-thread run passed all three tests. The single-thread regression
run passed all three selected tests. The exact build, identity, and test logs
are `/private/tmp/hyperray-logs-20260906/native-rebuild-build.log`,
`native-rebuild-evidence.txt`, `native-explicit-rebuilt.log`, and
`native-single-thread-rebuilt.log`. The rebuilt binaries remain at
`/private/tmp/hyperray-execution-guards-rebuild-20260906-0807/target/release/`.

The earlier missing-tool failures remain historical and are not overwritten:
they are recorded in the historical section above. Current coordinator review
is pending. This leaf still makes no full machine, language, scheduler, or
coverage claim.
