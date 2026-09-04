# Gates: Sail and Isla integration

Scope: Connect arbitrary bounded RISC-V program queries to Isla without instruction-specific Hyperray rules.

- [x] G1: The design defines the measured Sail-to-Isla route and separates a proposal from a proof.
  CHECK: rg -n 'This result remains a proposal until coverage accepts it' docs/isla-integration.md
  EXPECT: This result remains a proposal until coverage accepts it
  EVIDENCE: `docs/isla-integration.md` defines the route, result, error, coverage, and program-independence rules.

- [ ] G2: A public constructor records the Isla executable version and SHA-256 digest.
  EVIDENCE: pending

- [ ] G3: A public request accepts all artifact paths, expected digests, and finite resource limits.
  EVIDENCE: pending

- [ ] G4: One public operation returns typed proof proposals and typed engine errors.
  EVIDENCE: pending

- [ ] G5: A timeout, process error, malformed result, changed artifact, or visit-limit error cannot return a proposal.
  EVIDENCE: pending

- [ ] G6: Two different programs use the same production operation without a source change.
  EVIDENCE: pending

- [ ] G7: A correct claim returns no counterexample, and an incorrect claim returns a concrete counterexample.
  EVIDENCE: pending

- [ ] G8: The integration records artifact digests, bounds, tool identity, raw-output digest, and elapsed time.
  EVIDENCE: pending

- [ ] G9: All tests, formatters, static analysis, source limits, and the public API gate pass.
  EVIDENCE: pending
