# Gates: proof command

Scope: Let an external caller submit JSON and observe a verdict or engine error.

- [x] G1: A complete safe fixture prints PROVED.
  CHECK: go run ./cmd/hyperray prove testdata/proof/proved.json
  EXPECT: PROVED
  EVIDENCE: The result contained `"verdict": "PROVED"` and `"complete": true`.

- [x] G2: A failing fixture prints its exact counterexample.
  CHECK: go run ./cmd/hyperray prove testdata/proof/disproved.json
  EXPECT: DISPROVED
  EVIDENCE: The result contained `DISPROVED`, root `main-root`, and state `work`.

- [x] G3: An incomplete fixture exits unsuccessfully and names coverage.
  CHECK: go test ./cmd/hyperray -run TestIncompleteCoverageFails
  EXPECT: ok
  EVIDENCE: ok  github.com/HyperMarble/hyperray/cmd/hyperray  0.466s

- [x] G4: All command tests and checks pass.
  CHECK: go test ./cmd/hyperray && go vet ./cmd/hyperray
  EXPECT: ok
  EVIDENCE: ok  github.com/HyperMarble/hyperray/cmd/hyperray  0.113s

- [x] G5: Unknown JSON fields fail instead of being ignored.
  CHECK: go test ./cmd/hyperray -run TestUnknownJSONFieldFails
  EXPECT: ok
  EVIDENCE: ok  github.com/HyperMarble/hyperray/cmd/hyperray  0.103s
