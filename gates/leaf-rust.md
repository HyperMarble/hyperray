# Gates: Rust frontend

Scope: independent exact lowering of bounded Rust reference and verifier
semantics into shared IR.

- [ ] G1: Supported Rust branches, matches, bounded loops, calls, returns,
  Result/panic, effects/state lower through pinned compiler evidence.
  CHECK: go test ./tests -run '^TestFrontendRust$' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: Rust assertions and runner/pass logic lower independently into exact
  global Test IR inputs, including cross-case relationships.
  CHECK: go test ./tests -run '^TestFrontendRust(Test|Verifier|Relational)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Unsafe/FFI, uncontrolled state, unsupported macros or reachable
  untranslated compiler semantics return a precise blocker.
  CHECK: go test ./tests -run '^TestFrontendRustBlocked' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: Rust frontend has no mandatory adapter/generated-verifier path.
  CHECK: ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal/frontend/rust --glob '*.go'
  EXPECT: /^$/
  EVIDENCE: pending

