# Gates: Semantic IR and strict spec compiler

Scope: canonical independent finite models required by the frozen architecture.

- [ ] G1: Semantic IR represents complete finite domains, constraints,
  operations, observable outcomes/effects/state, provenance, exact reference
  behavior, one global Test predicate, environment, and translation coverage.
  CHECK: go test ./tests -run 'TestSemanticIR' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: Strict compilation expands complete full-N-way cases and rejects
  missing, overlapping, undeclared, unbounded, or prose-only semantics.
  CHECK: go test ./tests -run 'TestSpecCompiler|TestSpecLint' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Canonical IR/digests are deterministic and stale, incomplete, copied,
  or mismatched independent models fail closed.
  CHECK: go test ./tests -run 'TestSemanticIR(Canonical|Rejects|Independent|Digest)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: No generated-verifier or mandatory-adapter architecture remains in
  the shared IR or spec compiler.
  CHECK: ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal/semanticir internal/speccompiler --glob '*.go'
  EXPECT: /^$/
  EVIDENCE: pending

