# Gates: Isla event inventory

Scope: Record every retained Isla event before generic circuit translation.

- [x] G1: The design defines stable event records and exact translation coverage.
  CHECK: rg -n 'Isla event identifiers = translated event identifiers' docs/isla-event-coverage.md
  EXPECT: Isla event identifiers = translated event identifiers
  EVIDENCE: The design requires one disposition for every observed generic event.

- [x] G2: A public operation records every event in source order with a stable identifier.
  CHECK: go test -count=1 ./machine/isla -run TestPublicTraceEventCatalog
  EXPECT: ok
  EVIDENCE: The public catalog recorded four fixture events with exact address, trace, event, kind, and digest fields.

- [x] G3: The event reader accepts quoted values and Isla source annotations.
  CHECK: go test -count=1 ./machine/isla -run 'TestScanTraceOutputRecordsDirectEvents|TestCountTraceBlocksIgnoresQuotedDelimiters'
  EXPECT: ok
  EVIDENCE: Quoted delimiters, nested values, and source annotations did not add or remove events.

- [x] G4: Malformed syntax, empty events, and trace-level atoms return exact errors.
  CHECK: go test -count=1 ./machine/isla -run 'TestCountTraceBlocksRejectsMalformedOutput|TestTraceEventCatalogRejectsMalformedTrace|TestTraceEventHeadRejectsQuotedKind'
  EXPECT: ok
  EVIDENCE: Each malformed form returned a protocol error and no completed catalog.

- [x] G5: Input order and caller mutation cannot change a completed inventory.
  CHECK: go test -count=1 ./machine/isla -run 'TestTraceEventCatalogIgnoresCallerOrder|TestTraceEventCatalogOwnsItsValues'
  EXPECT: ok
  EVIDENCE: Reversed input and later caller mutations produced no catalog change.

- [x] G6: One real loaded ELF produces a complete event inventory for every instruction.
  CHECK: go test -count=1 -tags isla_integration ./machine/isla -run TestRealLoadedELFHasEveryInstructionTrace -v
  EXPECT: elf_traces=18 elf_events=15501 event_complete=true
  EVIDENCE: Four real instructions produced 18 traces and 15,501 inventoried events.

- [x] G7: Tests, formatting, static analysis, and source limits pass.
  CHECK: go test -count=1 -race -cover ./machine/isla && go test -count=1 ./... && go vet ./...
  EXPECT: coverage: 100.0% and all commands pass
  EVIDENCE: The Isla package measured 100.0 percent statement coverage. All Go packages and static analysis passed.
