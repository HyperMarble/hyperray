# Gates: compiler-built Rust constructs

Scope: Measure calls, dynamic dispatch, recursion, and local memory through the public executable proof API.
These fixtures do not establish full bounded-program coverage.

- [x] G1: The compiler retains the intended calls, dispatch, recursion, and memory effects.
  CHECK: env HYPERRAY_RUSTC=/Users/hak/.cargo/bin/rustc go test -count=1 -tags isla_integration ./machine/isla -run TestRealRustPatternCompilerEvidence -v
  EXPECT: --- PASS: TestRealRustPatternCompilerEvidence
  EVIDENCE: The command passed on 2026-09-05. Five native assertions passed. RV64 assembly retained two generic calls, one indirect call, one recursive call, and array loads and stores.

- [x] G2: The public proof API proves the declared results and rejects changed results for every fixture.
  EVIDENCE: All five cases passed both queries after shared unique-value resolution and separate instruction-visit limits. generic_calls passed in 60.11 seconds, dynamic_call in 66.76 seconds, recursive_calls in 161.17 seconds, stack_array in 124.86 seconds, and stack_values in 116.97 seconds. Their counterexamples were 20, 12, 6, 13, and 13. Their static/observed instruction counts were 23/23, 24/24, 23/23, 30/30, and 27/27. The full command exited zero in 530.486 seconds. /tmp/hyperray-unique-read.uazx9M/patterns.log retains the output. A separate real request with only two visits returned an error without a verdict in 66.12 seconds.
  Historical failures before the repair:
  After the entry fix, the generic-call experiment exceeded the host test
  timeout at 720.010 seconds. Its footprint subprocess lacked a host deadline.
  The shared deadline repair has a separate leaf. No call result follows from
  that terminated experiment. The fixture source and coverage rules remain.
  After the sequential-memory rebuild, stack_array passed both queries in
  95.75 seconds. stack_values passed both queries in 78.30 seconds.
  Generic calls, dynamic dispatch, and recursion each stopped with
  "instruction PC is not concrete". The retained run is
  /tmp/hyperray-sequential-replay.441HmB/calls.log. The shared address
  connection has a new leaf: gates/leaf-isla-symbolic-pc.md.
  After the shared terminal-boundary repair, generic_calls passed both queries
  in 63.68 seconds and dynamic_call passed both queries in 65.47 seconds.
  Their static and observed instruction counts were 23/23 and 24/24.
  Recursion still lacks a concrete PC on an executed instruction.
  The passing repeat is /tmp/hyperray-sequential-replay.441HmB/boundary-calls.log.

- [x] G3: Every failure names the missing production behavior without an excluded fixture.
  EVIDENCE: All five fixtures remain in TestRealRustPatterns. Their source and result requirements remain unchanged. The two-visit recursion request and invalid-stack request each returned explicit errors without verdicts. The shared memory mechanism contains no fixture-specific branch.

- [x] G4: New source files satisfy the format, analysis, and size rules.
  EVIDENCE: go vet with isla_integration passed. Gofmt and rustfmt reported no differences. Standalone Clippy with warnings as errors passed all five Rust fixtures. The largest new file contains 62 lines. An awk function-boundary measurement gives a maximum of 24 lines. Meaningful nesting does not exceed two levels.
