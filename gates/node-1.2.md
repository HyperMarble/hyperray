# Gates: independent translation integration

- [ ] G1: All three frontends and exact Test IR pass together without adapter
  or generated-verifier authority.
  CHECK: go test ./tests -run 'TestFrontend|TestTestIR' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

