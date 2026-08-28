# Gates: exact bounded proof engine

Scope: decide the frozen architecture's three UNSAT obligations and `T(C)`.

- [ ] G1: Reference query returns UNSAT only when every exact reference result
  is permitted, otherwise an exact `(x,o)` witness.
  CHECK: go test ./tests -run 'TestProofReference' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: False-positive query searches all complete behaviors `F` globally and
  returns a silently-passing wrong behavior when one exists.
  CHECK: go test ./tests -run 'TestProofFalsePositive' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: False-negative query searches all Spec-permitted complete behaviors
  and returns an unfairly rejected behavior when one exists.
  CHECK: go test ./tests -run 'TestProofFairness|TestProofFalseNegative' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: Exact reference acceptance proves `T(C)` independently from the
  reference-vs-Spec query.
  CHECK: go test ./tests -run 'TestProofReferenceAcceptance' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G5: Relational, multi-operation, effects, missing-case, allowed-alternative,
  and non-singleton adversaries cannot receive a false all-clear.
  CHECK: go test ./tests -run 'TestProof(Relational|MultiOperation|Effects|Flattened|Missing|Allowed|NonSingleton)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G6: Incomplete translation, non-finite models, unknown solver results, or
  missing global Test IR return `PROOF BLOCKED`.
  CHECK: go test ./tests -run 'TestProofBlocked' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

