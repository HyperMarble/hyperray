# Gates: Isla semantic trace artifacts

Scope: Retain complete Isla formulas and state events for generic circuit translation.

- [x] G1: The design defines the exact, lossless semantic trace route.
  CHECK: rg -n 'Sail behavior -> complete Isla event traces -> circuit behavior' docs/isla-integration.md
  EXPECT: Sail behavior -> complete Isla event traces -> circuit behavior
  EVIDENCE: The design uses Isla formulas and generic state events without instruction rules.

- [ ] G2: Each public instruction record exposes its exact trace output.
  EVIDENCE: pending

- [ ] G3: The validator recalculates each trace digest from the retained bytes and diagnostics.
  EVIDENCE: pending

- [ ] G4: Production trace arguments contain no simplification or hidden-event option.
  EVIDENCE: pending

- [ ] G5: A changed trace, missing trace, or partial operation returns an exact engine error.
  EVIDENCE: pending

- [ ] G6: One real loaded ELF retains lossless traces for every instruction.
  EVIDENCE: pending

- [ ] G7: Tests, formatting, static analysis, and source limits pass.
  EVIDENCE: pending
