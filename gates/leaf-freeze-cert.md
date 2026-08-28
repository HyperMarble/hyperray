# Gates: artifact freeze and proof certificate

Scope: bind the exact task, environment, IR, proofs, and confirmations.

- [ ] G1: Freeze hashes all required artifacts, repositories, workspaces,
  commands, pass signals, tools, dependencies, and environment and rejects
  drift.
  CHECK: go test ./tests -run 'TestFreeze' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: Certificate binds canonical Spec/reference/Test/environment IR, all
  four formal results, witnesses, confirmations, and tool identities.
  CHECK: go test ./tests -run 'TestCertificate' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Missing, stale, tampered, blocked, skipped, or solver-unknown evidence
  cannot issue or verify a `VERIFIED` certificate.
  CHECK: go test ./tests -run 'TestCertificate.*(Tamper|Reject|Blocked|Stale|Unknown)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

