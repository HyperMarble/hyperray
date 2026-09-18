# Gates: footprint host deadline

Scope: Enforce the declared footprint deadline outside the external tool.

- [x] G1: Source and live-process evidence explain the missing deadline.
  EVIDENCE: PID 43881 remained live for 9m19s with --timeout 120. The old runFootprint passed the caller context without a deadline. Pinned src/footprint.rs:612 passes a cooperative timeout to executor.rs:1042. docs/isla-footprint-deadline.md records this distinction. No live parent process was stopped by this task.

- [x] G2: A nonterminating footprint process returns a resource error within the host deadline.
  CHECK: go test -count=1 ./machine/isla -run '^TestFootprintHostDeadline$' -v
  EXPECT: --- PASS: TestFootprintHostDeadline
  EVIDENCE: PASS | ok  	github.com/HyperMarble/hyperray/machine/isla	1.962s

- [x] G3: Cancellation reaps the process and returns no partial instruction report.
  CHECK: go test -count=1 ./machine/isla -run '^TestFootprint(HostDeadline|CallerDeadlinePrecedesToolLimit)$' -v
  EXPECT: --- PASS: TestFootprintCallerDeadlinePrecedesToolLimit
  EVIDENCE: PASS | ok  	github.com/HyperMarble/hyperray/machine/isla	2.641s

- [x] G4: Overflow limits, ordinary reports, and package regressions retain explicit results.
  CHECK: go test -count=1 -race ./machine/isla && go vet ./machine/isla && printf 'footprint regression gates passed\n'
  EXPECT: footprint regression gates passed
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/machine/isla	13.454s | footprint regression gates passed
