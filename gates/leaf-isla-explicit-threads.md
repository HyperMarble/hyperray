# Gates: explicit Isla initial threads

Scope: connect ordered initial machine threads in one ELF to the existing
axiomatic Isla backend.

- [x] G1: Public one- and two-thread construction is deterministic, preserves
  caller slices, and retains ordered thread-entry identity.
- [x] G2: Invalid entries, zero threads, conflicting legacy fields, duplicate
  per-thread registers, sequential multi-thread requests, and output limits
  fail before native execution.
- [x] G3: Semantic evidence retains each thread's own entry event; count,
  identity, aggregate union, missing, extra, and duplicate inventory checks
  return explicit errors without a verdict.
- [x] G4: A real native two-thread parser/executor run observes both declared
  roots under axiomatic memory and produces a concrete changed-property
  counterexample.
- [x] G5: Equal-width 11-thread native ordering and branch-local first-PC
  checks pass, including independent thread 2 and thread 10 results.
- [x] G6: The compiler-built AtomicU64 fixture proves reader outputs are only
  0 or 1, reaches each value by separate counterexample queries, and observes
  one shared loaded ELF address in both thread traces.

This gate does not claim Rust spawn support, scheduler semantics, fairness, or
full machine/environment coverage.

Earlier recorded evidence (logs absent in the resumed environment): package, full-suite, and vet logs are under
`/private/tmp/hyperray-explicit-threads-go-test-machine-final.log`,
`/private/tmp/hyperray-explicit-threads-go-test-all-final2.log`, and
`/private/tmp/hyperray-explicit-threads-go-vet-final2.log`. Native execution
guards pass the two-thread, one-worker 11-thread, and AtomicU64 tests in
`/private/tmp/hyperray-explicit-threads-real-three-final.log`. The shared
trace records writer `write-mem` and reader `read-mem` at the same loaded
address; the concentrated-events negative test also passes.


## Historical resumed acceptance status: 2026-09-06, 07:46 UTC

The checked items record earlier evidence, not a fresh native acceptance pass.
The resumed session passed the focused negative tests, the package tests,
`go test ./... -count=1`, and `go vet ./...`. The public negative tests use
accepted baselines and exact errors. Parser-to-join tests cover both sibling
orders, equal-entry branches, and a shared instruction prefix.

- [ ] Fresh G4-G6 native acceptance: The three selected tests stopped with
  `tool_not_found` because the pinned native binaries are absent.
- [ ] Fresh single-thread native regressions: The selected executable,
  sequential-memory, and typed-state tests stopped with the same error.
- [ ] Coordinator review of the final test repairs.

The isolated concentrated-memory-events negative test passed. These native
failures remain historical and separate from passing Go checks. Exact commands and log names are in
`docs/isla-explicit-threads.md`, under "Historical resumed measurements". Log names start
with `/private/tmp/hyperray-explicit-resume-`. No native tool was rebuilt or
replaced with a different binary during that historical attempt.

## Current rebuilt acceptance status: 2026-09-06, 08:17 UTC

The pinned native tools were rebuilt from source commit
`7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6` with
`tools/isla/patches/execution-guards-v1.patch`, SHA-256
`fd5c8b12d4cb45b1405ed3f8a814c5de06eb39eb5c7aad11938d5eec10bf6af8`. The
isolated locked offline build used `CARGO_BUILD_JOBS=2` and completed with
exit status 0 in task `025373gj62`.

- [x] Fresh G4-G6 native acceptance: `TestRealRustExplicitThreads`,
  `TestRealRustExplicitThreadOrdering`, and `TestRealRustExplicitSharedAtomic`
  passed with the rebuilt tools.
- [x] Fresh single-thread native regressions: `TestRealRustExecutablePrograms`,
  `TestRealRustSequentialMixedWidth`, and `TestRealRustTypedTrapState` passed.
- [x] Fresh Go validation: `go test ./... -count=1`, `go vet ./...`,
  formatting, and whitespace checks passed.
- [x] Coordinator review of the final test repairs and this evidence update.
  At 08:20 UTC, the coordinator inspected the native logs, full Go results,
  focused negative tests, and current binary hashes. Independent reviewer
  `session_crocodile_1788681659479_c47613b8d9585903` closed both P2 findings
  on the 08:19:11 UTC source snapshot. No issue remains within that review scope.

The rebuilt tools report the execution-guards-v1 version suffix. Their measured
SHA-256 identities are `isla-axiomatic`
`8da83833f3d2a32b67f6bcbf31e46934757077172580d517e414265a496effaf`,
`isla-litmus-dump`
`012648a9b3288e5d297bf83b5735ed3af9eaa744c9d538c340dc8eb053e11deb`, and
`isla-footprint`
`c736e69bfc092b262270c20f5e148760a4e11b7ac4dca1ab14a1adfe62800d58`.
The exact commands and compact logs are in
`/private/tmp/hyperray-logs-20260906/native-rebuild-build.log`,
`native-rebuild-evidence.txt`, `native-explicit-rebuilt.log`,
`native-single-thread-rebuilt.log`, `go-test-all-after-p2.log`, and
`go-vet-after-p2.log`. The rebuilt tools are in
`/private/tmp/hyperray-execution-guards-rebuild-20260906-0807/target/release/`.

This updates current native acceptance only. It does not convert the
historical missing-tool failures into passes, and it does not claim full
machine, language, scheduler, or coverage support.
