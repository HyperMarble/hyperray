# Gates: Sail and Isla integration

Scope: Connect arbitrary bounded RISC-V program queries to Isla without instruction-specific Hyperray rules.

- [x] G1: The design defines the measured Sail-to-Isla route and separates a proposal from a proof.
  CHECK: rg -n 'This result remains a proposal until coverage accepts it' docs/isla-integration.md
  EXPECT: This result remains a proposal until coverage accepts it
  EVIDENCE: `docs/isla-integration.md` defines the route, result, error, coverage, and program-independence rules.

- [x] G2: A public constructor records the Isla executable version and SHA-256 digest.
  CHECK: go test -count=1 ./machine/isla -run TestPublicEngineIdentity
  EXPECT: ok
  EVIDENCE: The external test operated an executable, measured its version and digest, and observed both through `Engine.Identity`.

- [x] G3: A public request accepts all artifact paths, expected digests, and finite resource limits.
  CHECK: go test -count=1 ./machine/isla -run 'TestPublicRequest|TestRequestRejectsZeroLimits'
  EXPECT: ok
  EVIDENCE: External tests construct all four artifacts and both finite limits through `NewRequest`.

- [x] G4: One public operation returns typed proof proposals and typed engine errors.
  CHECK: go test -count=1 ./machine/isla -run 'TestDifferentProgram|TestCorrectProgram|TestOperationErrors'
  EXPECT: ok
  EVIDENCE: `Engine.Propose` returns one typed proposal or one typed `isla.Error`.

- [x] G5: A timeout, process error, malformed result, changed artifact, or visit-limit error cannot return a proposal.
  CHECK: go test -count=1 ./machine/isla -run 'TestOperationErrors|TestCanceledContext|TestChanged|TestRemoved'
  EXPECT: ok
  EVIDENCE: Every tested failure returned a stable error code and no proposal.

- [x] G6: Two different programs use the same production operation without a source change.
  CHECK: go test -count=1 ./machine/isla -run 'TestDifferentProgramFindsCounterexample|TestCorrectProgramHasNoCounterexample'
  EXPECT: ok
  EVIDENCE: The same `Engine.Propose` operation accepted two program artifacts and returned their different results.

- [x] G7: A correct claim returns no counterexample, and an incorrect claim returns a concrete counterexample.
  CHECK: go test -count=1 -tags isla_integration ./machine/isla -run TestRealIslaReturnsBothResults
  EXPECT: proof=no_counterexample_found counterexample=counterexample_found state=0:x5=#x0000000000000003;
  EVIDENCE: The public API passed with Isla v0.2.0. It returned both results and the exact `x5` counterexample state.

- [x] G8: The integration records artifact digests, bounds, tool identity, raw-output digest, and elapsed time.
  CHECK: go test -count=1 ./machine/isla -run TestCorrectProgramHasNoCounterexample
  EXPECT: ok
  EVIDENCE: The public proposal exposes six SHA-256 values, two limits, the tool version, diagnostics, and elapsed milliseconds.

- [x] G9: All tests, formatters, static analysis, source limits, and the public API gate pass.
  CHECK: go test -count=1 ./... && go vet ./...
  EXPECT: ok
  EVIDENCE: All Go packages passed. The Isla package measured 100.0% statement coverage. All changed files contain 75 lines or fewer.
