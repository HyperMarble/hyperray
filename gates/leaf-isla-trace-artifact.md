# Gates: Isla semantic trace artifacts

Scope: Retain complete Isla formulas and state events for generic circuit translation.

- [x] G1: The design defines the exact, lossless semantic trace route.
  CHECK: rg -n 'Sail behavior -> complete Isla event traces -> circuit behavior' docs/isla-integration.md
  EXPECT: Sail behavior -> complete Isla event traces -> circuit behavior
  EVIDENCE: The design uses Isla formulas and generic state events without instruction rules.

- [x] G2: Each public instruction record exposes its exact trace output.
  CHECK: go test -count=1 ./machine/isla -run TestPublicFootprintRequestAndOperation
  EXPECT: ok
  EVIDENCE: The public instruction records contained the trace text from the identified tool.

- [x] G3: The validator recalculates each trace digest from the retained bytes and diagnostics.
  CHECK: go test -count=1 ./machine/isla -run TestFootprintCoverageRejectsInventoryMismatch
  EXPECT: ok
  EVIDENCE: Independent output and digest mutations stopped coverage acceptance.

- [x] G4: Production trace arguments contain no simplification or hidden-event option.
  CHECK: '! rg -n -- ''"-s"|--simplify-registers|--hide'' machine/isla/footprint_arguments.go'
  EXPECT: no output
  EVIDENCE: The production argument scan found no lossy trace option.

- [x] G5: A changed trace, missing trace, or partial operation returns an exact engine error.
  CHECK: go test -count=1 ./machine/isla -run 'TestFootprintCoverageRejectsInventoryMismatch|TestFootprintOperationRejectsToolResults'
  EXPECT: ok
  EVIDENCE: Trace mutations returned coverage errors. Process and protocol errors returned no partial report.

- [x] G6: One real loaded ELF retains lossless traces for every instruction.
  CHECK: go test -count=1 -tags isla_integration ./machine/isla -run TestRealLoadedELFHasEveryInstructionTrace
  EXPECT: elf_instructions=4 covered=4 complete=true
  EVIDENCE: The public route retained and accepted unsimplified traces for all four instructions.

- [x] G7: Tests, formatting, static analysis, and source limits pass.
  CHECK: go test -count=1 -race -cover ./machine/isla && go test -count=1 ./... && go vet ./...
  EXPECT: coverage: 100.0% and all commands pass
  EVIDENCE: The Isla package measured 100.0 percent statement coverage. All Go packages and static analysis passed.
