# Gates: language-neutral reference kernel

Scope: Integrate the model, coverage, proof, and CLI leaves.

- [x] G1: Every reference-kernel package test passes.
  CHECK: go test ./model ./coverage ./proof ./cmd/hyperray
  EXPECT: ok
  EVIDENCE: On 2026-09-05 all four packages returned `ok` with uncached runs.

- [x] G2: The whole Go module passes static analysis.
  CHECK: go vet ./...
  EXPECT: /^$/
  EVIDENCE: On 2026-09-05 `go vet ./...` returned exit 0 with no output.

- [x] G3: The public API is reachable, buildable, callable, and observable from an external package test.
  EVIDENCE: External `model_test`, `coverage_test`, and `proof_test` packages construct public values, call `model.Validate`, `coverage.Check`, and `proof.Check`, and observe errors, reports, verdicts, reachability, and witnesses. The uncached G1 run passed these tests on 2026-09-05.
