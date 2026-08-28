# Gates: production pipeline and CLI

Scope: one fail-closed `ray start`/`ray check` path implementing the frozen
whole flow.

- [ ] G1: Pipeline orders freeze, spec compile, diagnostics, independent
  reference/Test translation, exact proofs, confirmations, and certificate.
  CHECK: go test ./tests -run 'TestPipelineStages' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: `ray start` and `ray check` use the same pipeline and return nonzero
  for `NOT VERIFIED` and `PROOF BLOCKED`.
  CHECK: go test ./tests -run 'TestCLI' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: PICT/mutation/oracle/diff diagnostics cannot manufacture `VERIFIED`;
  blocked, skipped, stale, unsupported, or unknown mandatory stages fail closed.
  CHECK: go test ./tests -run 'TestPipeline(IgnoresMutationForProof|Rejects|Blocked|Unknown|Stale)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: Pipeline contains no generated replacement verifier or mandatory
  adapter stage.
  CHECK: ! rg -n 'GeneratedVerifier|AdapterBoundary|AdapterClosure|FiniteAdapter|ray-adapter-v1' internal/pipeline cmd/ray --glob '*.go'
  EXPECT: /^$/
  EVIDENCE: pending

