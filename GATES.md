# Gates: Ray v0.10 complete bounded-task verifier

Scope: production `ray start`/`ray check` for frozen bounded Python, Rust, and
C++ coding tasks or PRs, exactly as defined by the hash-frozen architecture.

- [ ] G0: The authoritative architecture remains byte-for-byte frozen and the
  implementation contains no production dependency on the rejected generated
  verifier or mandatory adapter architecture.
  CHECK: cd docs/specs && shasum -a 256 -c architecture-freeze.sha256 && cd ../.. && ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal cmd --glob '*.go'
  EXPECT: /whole flow.md: OK/
  EVIDENCE: pending

- [ ] G1: Task instruction, base, spec, solution, tests, commands, pass signal,
  tools, dependencies, and environment are deterministically frozen and
  tampering or stale evidence is rejected.
  CHECK: go test ./tests -run 'TestFreeze|TestCertificate' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G2: `spec.md` compiles into canonical complete full-N-way Spec Semantic
  IR with exact finite domains, constraints, outcomes, effects, provenance,
  disjointness, completeness, and a stable digest.
  CHECK: go test ./tests -run 'TestSemanticIR|TestSpecCompiler|TestSpecLint' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G3: Python, Rust, and C++ frontends independently translate the real
  frozen reference and verifier semantics for accepted bounded fixtures and
  fail closed on unsupported reachable constructs.
  CHECK: go test ./tests -run 'TestFrontend(Python|Rust|CPP)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G4: Test IR exactly represents the complete frozen verifier as one
  global predicate over the whole behavior vector, preserving conjunctions,
  cross-case comparisons, ordering, shared state, and final pass signal.
  CHECK: go test ./tests -run 'TestTestIR' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G5: The proof engine exactly proves or refutes reference correctness,
  false-positive freedom, false-negative freedom, and exact reference
  acceptance over the complete bounded behavior space.
  CHECK: go test ./tests -run 'TestProof' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G6: Adversarial proof fixtures cover missing cases, weak assertions,
  allowed alternatives, relational tests, interacting operations, exception
  boundaries, omitted hidden requirements, and non-singleton domains without a
  false `VERIFIED` result.
  CHECK: go test ./tests -run 'TestProof(Relational|MultiOperation|Effects|Flattened|FalsePositive|Fairness|Reference)|TestPipeline(Missing|Allowed|Relational|Hidden|Boundary)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G7: Every formal witness is confirmed in a fresh copy of the real frozen
  environment, and source/workspace bytes are restored after success, failure,
  timeout, and cancellation.
  CHECK: go test ./tests -run 'TestCounterexampleExecutor' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G8: Existing Ray diagnostic layers remain wired where applicable, but
  PICT, mutation, fuzzing, property-based tests, coverage, and differential
  samples cannot change a formal verdict to `VERIFIED`.
  CHECK: go test ./tests -run 'TestCoverage|TestOracle|TestDiffTest|TestDepHarvest|TestPipelineIgnoresMutationForProof' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G9: `ray start` and `ray check` execute the same fail-closed pipeline;
  blocked, unknown, skipped, stale, or failed proof stages return nonzero and
  cannot issue a valid certificate.
  CHECK: go test ./tests -run 'TestPipeline|TestCLI' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G10: Real end-to-end Python, Rust, and C++ bounded tasks reach the correct
  verdict, and negative variants expose exact reference, false-positive, and
  false-negative witnesses.
  CHECK: go test ./tests -run 'TestE2E' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G11: The complete repository test suite passes without panic, skipped
  production assertions, or `[no tests to run]` being counted as evidence.
  CHECK: go test ./... -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

- [ ] G12: Static analysis passes across the repository.
  CHECK: go vet ./...
  EXPECT: /^$/
  EVIDENCE: pending

- [ ] G13: A final adversarial audit finds no path from incomplete evidence,
  copied spec results, sampling, or task-specific production logic to
  `VERIFIED`.
  EVIDENCE: pending
