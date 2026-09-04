# Gates: Isla event inventory

Scope: Record every retained Isla event before generic circuit translation.

- [x] G1: The design defines stable event records and exact translation coverage.
  CHECK: rg -n 'Isla event identifiers = translated event identifiers' docs/isla-event-coverage.md
  EXPECT: Isla event identifiers = translated event identifiers
  EVIDENCE: The design requires one disposition for every observed generic event.

- [ ] G2: A public operation records every event in source order with a stable identifier.
  EVIDENCE: pending

- [ ] G3: The event reader accepts quoted values and Isla source annotations.
  EVIDENCE: pending

- [ ] G4: Malformed syntax, empty events, and trace-level atoms return exact errors.
  EVIDENCE: pending

- [ ] G5: Input order and caller mutation cannot change a completed inventory.
  EVIDENCE: pending

- [ ] G6: One real loaded ELF produces a complete event inventory for every instruction.
  EVIDENCE: pending

- [ ] G7: Tests, formatting, static analysis, and source limits pass.
  EVIDENCE: pending
