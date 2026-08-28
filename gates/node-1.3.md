# Gates: proof and confirmation integration

- [ ] G1: Exact proofs and real witness confirmation pass together.
  CHECK: go test ./tests -run 'TestProof|TestCounterexampleExecutor' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

