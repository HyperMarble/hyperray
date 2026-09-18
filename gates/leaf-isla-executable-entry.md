# Gates: executable entry preservation

Scope: Preserve executable placement without assuming the entry is first.

- [ ] G1: The regression exposes a pre-entry function overlap before the fix.
  EVIDENCE: pending
- [ ] G2: Generated program sections retain every original byte and address.
  EVIDENCE: pending
- [ ] G3: The pinned tool starts at the declared ELF entry.
  EVIDENCE: pending
- [ ] G4: Old tool support, malformed entries, and placement changes fail explicitly.
  EVIDENCE: pending
- [ ] G5: Real compiler fixtures and regression analysis pass.
  EVIDENCE: pending
