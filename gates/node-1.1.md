# Gates: semantic foundation integration

- [ ] G1: IR, compiler, freeze, skill, and certificate tests pass together.
  CHECK: go test ./tests -run 'TestSemanticIR|TestSpecCompiler|TestSpecSkill|TestFreeze|TestCertificate' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

