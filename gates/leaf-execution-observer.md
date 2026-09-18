# Gates: experimental execution observer

Scope: A reusable public execution observer and CLI, separate from proof verdicts.

- [x] G1: Public API tests cover input, process, output, and resource decisions.
  CHECK: go test -count=1 ./execution ./cmd/hyperray && echo OBSERVER_API_OK
  EXPECT: OBSERVER_API_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/cmd/hyperray	0.194s | OBSERVER_API_OK

- [x] G2: Every declared native and thread integration case uses the same SDK.
  CHECK: go test -count=1 -tags execution_integration -run TestResearchMatrix -v ./execution && echo OBSERVER_MATRIX_OK
  EXPECT: OBSERVER_MATRIX_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution	1.331s | OBSERVER_MATRIX_OK

- [x] G3: Go regression tests and static analysis pass.
  CHECK: go test ./... && go vet ./... && echo OBSERVER_REGRESSION_OK
  EXPECT: OBSERVER_REGRESSION_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/tools/sail-jib/go	(cached) | OBSERVER_REGRESSION_OK

- [x] G4: Cancellation and bounded output pass the race detector.
  CHECK: go test -race -count=1 ./execution ./cmd/hyperray && echo OBSERVER_RACE_OK
  EXPECT: OBSERVER_RACE_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/cmd/hyperray	1.435s | OBSERVER_RACE_OK

- [x] G5: Formatting, source limits, public construction, and verdict separation pass review.
  EVIDENCE: gofmt reported no files. The source audit measured 20 Go files, at most 57 lines per file and 29 lines per function. External-package tests construct Request and call Run. No result contains a proof verdict. Linux cross-compilation and the CLI build returned exit 0. The local macOS manual specifies resident-memory bytes.
