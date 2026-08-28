# Gates: Python frontend

Scope: independent exact lowering of bounded Python reference and verifier
semantics into shared IR.

- [ ] G1: Supported Python branches, loops with proven bounds, calls, returns,
  exceptions, values/types/shapes/effects/state lower with exact provenance.
  CHECK: go test ./tests -run '^TestFrontendPython$' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: Python test assertions and pass logic lower independently into exact
  global Test IR inputs, including cross-case relationships.
  CHECK: go test ./tests -run '^TestFrontendPython(Test|Verifier|Relational)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Reflection, uncontrolled state, unsupported dynamic behavior, or any
  reachable untranslated construct returns a precise blocker.
  CHECK: go test ./tests -run '^TestFrontendPythonBlocked' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: Python frontend has no mandatory adapter/generated-verifier path.
  CHECK: ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal/frontend/python --glob '*.go'
  EXPECT: /^$/
  EVIDENCE: pending

