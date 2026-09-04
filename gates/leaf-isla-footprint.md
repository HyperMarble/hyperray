# Gates: Isla instruction coverage

Scope: Bind every loaded instruction to semantics from one pinned Sail and Isla release set.

- [x] G1: The design defines the generic instruction trace route and its limits.
  CHECK: rg -n 'Hyperray sends each loaded instruction' docs/isla-integration.md
  EXPECT: Hyperray sends each loaded instruction
  EVIDENCE: The design binds bytes and addresses to Isla traces without Hyperray opcode rules.

- [x] G2: A public engine records the exact `isla-footprint` identity.
  CHECK: go test -count=1 ./machine/isla -run TestPublicFootprintEngineIdentity
  EXPECT: ok
  EVIDENCE: The public constructor ran a real executable and exposed its path, version, and SHA-256 digest.

- [x] G3: A public request accepts identified model artifacts, all instructions, and finite limits.
  CHECK: go test -count=1 ./machine/isla -run 'TestPublicFootprintRequest|TestFootprintRequestRejects'
  EXPECT: ok
  EVIDENCE: External tests construct the full request and observe rejected inventories and limits.

- [x] G4: The operation returns one trace record for every instruction address and encoding.
  CHECK: go test -count=1 ./machine/isla -run TestPublicFootprintRequestAndOperation
  EXPECT: ok
  EVIDENCE: One public operation returned two ordered records with exact addresses, encodings, trace counts, and distinct digests.

- [x] G5: Tool, artifact, timeout, trace, or inventory failures return exact engine errors.
  CHECK: go test -count=1 ./machine/isla -run 'TestFootprintOperationRejects|TestZeroFootprint'
  EXPECT: ok
  EVIDENCE: Process, protocol, output, context, changed-tool, and unidentified-tool failures returned no partial report.

- [ ] G6: Real ADDI and illegal-encoding measurements produce distinct Sail semantics.

- [ ] G7: Production code contains no fixture address, instruction bytes, or instruction name.

- [ ] G8: Tests, formatting, static analysis, source limits, and the public API gate pass.
