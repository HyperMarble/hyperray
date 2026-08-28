# Gates: C++ frontend

Scope: independent exact lowering of bounded C++ reference and verifier
semantics into shared IR.

- [ ] G1: Supported C++ branches, switches, bounded loops, calls, returns,
  exceptions, effects/state lower through pinned Clang/LLVM evidence.
  CHECK: go test ./tests -run '^TestFrontendCPP$' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: C++ assertions and runner/pass logic lower independently into exact
  global Test IR inputs, including cross-case relationships.
  CHECK: go test ./tests -run '^TestFrontendCPP(Test|Verifier|Relational)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Undefined behavior, inline assembly, uncontrolled state, unsupported
  templates or reachable untranslated compiler semantics return a blocker.
  CHECK: go test ./tests -run '^TestFrontendCPPBlocked' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: C++ frontend has no mandatory adapter/generated-verifier path.
  CHECK: ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal/frontend/cpp --glob '*.go'
  EXPECT: /^$/
  EVIDENCE: pending

