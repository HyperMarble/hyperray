# Gates: product integration

- [ ] G1: Pipeline, CLI, and all end-to-end language tasks pass together.
  CHECK: go test ./tests -run 'TestPipeline|TestCLI|TestE2E' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
