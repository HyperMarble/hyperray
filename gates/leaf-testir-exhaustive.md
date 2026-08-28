# Gates: exact global Test IR

Scope: translate the real frozen verifier and pass signal into exact `T(F)`.

- [ ] G1: Multiple tests combine by their real global logic and preserve
  relational, cross-case, ordering, shared-state, and interaction assertions.
  CHECK: go test ./tests -run 'TestTestIR(Relational|Global|Intersection|Ordering|State|Interaction)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: The complete finite behavior-vector space is represented by exact
  enumeration or symbolic formula; caps and samples block rather than certify.
  CHECK: go test ./tests -run 'TestTestIR(Exhaustive|Symbolic|ResourceLimit|NoSampling)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Test IR is derived from real test/verifier code and authoritative pass
  signal, not names, words, coverage, spec outcomes, or generated tests.
  CHECK: go test ./tests -run 'TestTestIR(Evidence|Independent|PassSignal|Rejects)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: Missing or incomplete test translation, runner selection, or
  environment evidence blocks verification.
  CHECK: go test ./tests -run 'TestTestIR.*Blocked' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G5: Test IR contains no generated replacement verifier or mandatory
  adapter authority.
  CHECK: ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal/testir --glob '*.go'
  EXPECT: /^$/
  EVIDENCE: pending

