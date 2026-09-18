# Gates: frozen proof contract

Scope: Integrate the architecture text and its versioned artifact schemas.

- [x] G1: The architecture document gates pass.
  CHECK: node /Volumes/Hak_SSD/.agents/skills/unlazy/scripts/gate-check.mjs --status gates/leaf-design.md
  EXPECT: 6/6 gates met
  EVIDENCE: Parent rerun reports `ALL MET (6 met)`.

- [x] G2: The schema gates pass.
  CHECK: node /Volumes/Hak_SSD/.agents/skills/unlazy/scripts/gate-check.mjs --status gates/leaf-schemas.md
  EXPECT: 7/7 gates met
  EVIDENCE: Parent rerun reports `ALL MET (7 met)`.

- [x] G3: Schema field meanings match the prose contract.
  EVIDENCE: Parent inspection matched boundary, model, requirement, coverage, result, and summary fields to `docs/proof-machine.md`.
