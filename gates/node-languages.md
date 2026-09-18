# Gates: source language routes

Scope: Integrate Rust, C, C++, Go, and Python with the same proof machine.

- [ ] G1: Every language leaf gate passes.
  EVIDENCE: pending

- [ ] G2: No language-specific prover is required for semantic coverage.
  EVIDENCE: pending

- [x] G3: Rust, C, C++, Go, and Python use separate adapter directories, and
  shared machine or proof logic does not live in a language adapter.
  CHECK: go test -count=1 ./layout
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/layout	0.402s
  has production source, so the full five-language gate remains open.

- [ ] G4: A changed source program or finite boundary uses the same adapter
  binary and generates new artifacts without a Hyperray source change.
  EVIDENCE: pending
