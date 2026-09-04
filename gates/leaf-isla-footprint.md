# Gates: Isla instruction coverage

Scope: Bind every loaded instruction to semantics from one pinned Sail and Isla release set.

- [x] G1: The design defines the generic instruction trace route and its limits.
  CHECK: rg -n 'Hyperray sends each loaded instruction' docs/isla-integration.md
  EXPECT: Hyperray sends each loaded instruction
  EVIDENCE: The design binds bytes and addresses to Isla traces without Hyperray opcode rules.

- [ ] G2: A public engine records the exact `isla-footprint` identity.

- [ ] G3: A public request accepts identified model artifacts, all instructions, and finite limits.

- [ ] G4: The operation returns one trace record for every instruction address and encoding.

- [ ] G5: Tool, artifact, timeout, trace, or inventory failures return exact engine errors.

- [ ] G6: Real ADDI and illegal-encoding measurements produce distinct Sail semantics.

- [ ] G7: Production code contains no fixture address, instruction bytes, or instruction name.

- [ ] G8: Tests, formatting, static analysis, source limits, and the public API gate pass.
