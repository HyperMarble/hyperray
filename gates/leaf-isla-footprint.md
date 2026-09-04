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

- [x] G6: Real ADDI and illegal-encoding measurements produce distinct Sail semantics.
  CHECK: HYPERRAY_ISLA_FOOTPRINT=... HYPERRAY_SAIL_IR=... HYPERRAY_ISLA_CONFIG=... go test -count=1 -tags isla_integration ./machine/isla -run TestRealFootprintTracesLegalAndIllegalEncodings -v
  EXPECT: traces=2 legal=... illegal=...
  EVIDENCE: The recorded Isla engine returned one trace for each input. Their SHA-256 digests differed. The run took 1.31 seconds.

- [x] G7: Production code contains no fixture address, instruction bytes, or instruction name.
  CHECK: ! rg -n 'ADDI|ILLEGAL|93023000|ffffffff|0x1000' machine/isla --glob 'footprint_*.go' --glob '!*_test.go'
  EXPECT: no output
  EVIDENCE: The production scan found no test opcode, address, or instruction-family name.

- [x] G8: Tests, formatting, static analysis, source limits, and the public API gate pass.
  CHECK: go test -count=1 -race -cover ./machine/isla && go test -count=1 ./... && go vet ./...
  EXPECT: coverage: 100.0% and all commands pass
  EVIDENCE: The package measured 100.0 percent statement coverage. All Go packages and static analysis passed.

- [ ] G9: One release manifest binds the engine, Sail snapshot, and configuration digests.

- [ ] G10: An independent validator reconciles the request inventory and trace inventory in both directions.

- [ ] G11: Every Isla diagnostic has a proved unreachable disposition or stops coverage acceptance.

- [ ] G12: One real loaded ELF passes from `machine.Load` through every instruction trace.
