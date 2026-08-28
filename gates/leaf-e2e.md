# Gates: real end-to-end task/PR validation

Scope: production CLI proof on bounded Python, Rust, and C++ task bundles.

- [ ] G1: Python positive reaches `VERIFIED`; reference-bug, missing-test, and
  unfair-hidden-test variants return exact witnesses.
  CHECK: go test ./tests -run 'TestE2EPython' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: Rust positive reaches `VERIFIED`; reference-bug, missing-test, and
  unfair-hidden-test variants return exact witnesses.
  CHECK: go test ./tests -run 'TestE2ERust' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: C++ positive reaches `VERIFIED`; reference-bug, missing-test, and
  unfair-hidden-test variants return exact witnesses.
  CHECK: go test ./tests -run 'TestE2ECPP' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: At least one reconstructed real task or PR runs through production,
  and its frozen provenance is checked rather than replaced by a toy function.
  CHECK: go test ./tests -run 'TestE2ERealTask' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G5: Stale/tampered/unsupported task variants cannot return `VERIFIED`.
  CHECK: go test ./tests -run 'TestE2E.*(Stale|Tamper|Blocked)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

## In-progress blocker log (not gate evidence)

- 2026-08-27: Removed 477 files from 13 obsolete E2E fixture families that
  depended on the rejected Phase-A/adapter-era configuration or narrowed
  unbounded real-task semantics. Retained only the byte-exact jcode
  reconstruction, rewritten as ordinary instruction/base/solution/test
  provenance.
- 2026-08-27: The first corrected-architecture provenance rerun was blocked by
  concurrent adapter removal in production packages (`InputUniverse` remained
  in Python/C++ while Semantic IR had removed it; certificate still referenced
  `AdapterBoundary`). This is shared integration status, not a leaf pass.
- 2026-08-27: Final strict spec grammar, mandatory diagnostic/oracle inputs,
  public+hidden multi-artifact runner closure, and exact stage names are still
  landing. No fixture will claim `VERIFIED` against a transitional schema.
