# Gates: same-program Isla verification

Scope: Return a verdict only after one pinned Isla profile analyzes the same bounded program for semantics and correctness.

- [x] G1: One public request creates both Isla operations from the same identified artifacts.
  CHECK: go test -count=1 ./machine/isla -run TestPublicVerifierReturnsBothResults
  EXPECT: ok
  EVIDENCE: `VerificationRequest` contains one solver query. The verifier derives the semantic operation from that query.

- [x] G2: A safe fixture returns `PROVED` with matching program and model digests.
  CHECK: go test -count=1 ./machine/isla -run TestPublicVerifierReturnsBothResults
  EXPECT: ok
  EVIDENCE: The safe fixture returned `PROVED`. Its semantic and solver program digests were equal.

- [x] G3: An unsafe fixture returns `DISPROVED` with its solver state and semantic trace.
  CHECK: go test -count=1 ./machine/isla -run TestPublicVerifierReturnsBothResults
  EXPECT: ok
  EVIDENCE: The unsafe fixture returned `DISPROVED`, a nonempty solver state, and one instruction event.

- [x] G4: Missing, extra, duplicate, or malformed semantic records return no verdict.
  CHECK: go test -count=1 ./machine/isla -run 'TestVerifierRejectsSemanticCoverageFailures|TestSemanticParserRejects'
  EXPECT: ok
  EVIDENCE: All incomplete, unequal, duplicate, malformed, and reordered reports returned typed errors and zero results.

- [x] G5: A changed artifact, changed tool, version mismatch, resource failure, or unknown diagnostic returns no verdict.
  CHECK: go test -count=1 ./machine/isla -run 'TestVerifierRejects|TestSolverResourceLimits|TestProposalRejectsUnknown'
  EXPECT: ok
  EVIDENCE: Every identity, mutation, resource, diagnostic, and process failure returned an error and a zero result.

- [x] G6: The pinned real Isla release passes both fixtures through the public verifier.
  CHECK: HYPERRAY_ISLA=/Volumes/Hak_SSD/hyperray-research/isla-7f6882b/target/release/isla-axiomatic HYPERRAY_ISLA_DUMP=/Volumes/Hak_SSD/hyperray-research/isla-7f6882b/target/release/isla-litmus-dump HYPERRAY_SAIL_IR=/Volumes/Hak_SSD/hyperray-research/isla-snapshots/riscv_model_rv64d.ir HYPERRAY_ISLA_CONFIG=/Volumes/Hak_SSD/hyperray-research/isla-7f6882b/configs/riscv64.toml HYPERRAY_MEMORY_MODEL=/Volumes/Hak_SSD/hyperray-research/isla-7f6882b/web/client/dist/riscv.cat go test -count=1 -tags isla_integration ./machine/isla -run TestRealIslaSameProgramResults -v
  EXPECT: proof=PROVED counterexample=DISPROVED instructions=1
  EVIDENCE: The pinned Isla `v0.2.0/z3-5.1.0.0` tools passed both real ADDI programs in 7.04 seconds.

- [x] G7: Tests, formatting, static analysis, source limits, and the public API gate pass.
  CHECK: go test -count=1 -race -cover ./machine/isla && go test -count=1 ./... && go vet ./...
  EXPECT: coverage: 100.0% and all commands pass
  EVIDENCE: The Isla package measured 100.0% statement coverage. All Go packages and static analysis passed.
