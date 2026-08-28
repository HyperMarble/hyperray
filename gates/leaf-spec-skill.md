# Gates: trustworthy machine-readable spec authoring

Scope: author complete bounded `spec.md` from the whole fixed task before tests
are used as enforcement evidence.

- [ ] G1: The skill derives semantics from instruction/issue, base code,
  reference diff, changed branches/call paths, and frozen environment before
  inspecting tests.
  CHECK: go test ./tests -run 'TestSpecSkillWorkflow' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: OPEN (2026-08-27) — owned-file equivalent passes: `go test tests/spec_skill_test.go -run 'TestSpecSkillWorkflow' -count=1` → `ok command-line-arguments 0.144s`. The exact package command is temporarily compile-blocked by corrected-architecture migration in non-owned `internal/pipeline` (`certificate.go` still references removed snapshot/acceptance fields, `diagnostics.go` passes the old environment type, and `run.go` references removed adapter binding); the pipeline owner was notified.
- [ ] G2: It defines exact domains, impossible-case constraints, full-N-way
  required outcomes/effects/state, provenance, patch-shaped tasks, and the
  separate prompt/test mismatch review.
  CHECK: go test ./tests -run 'TestSpecSkillSchema|TestSpecSkillTemplate' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: OPEN (2026-08-27) — owned-file equivalent passes: `go test tests/spec_skill_test.go -run 'TestSpecSkillSchema|TestSpecSkillTemplate' -count=1` → `ok command-line-arguments 0.133s`. The corrected template compiles to canonical Spec Semantic IR with two operation-local 2×2 products, seven reachable requirements, one exact constraint, closed outcomes/effects, provenance, no Test IDs, and no copied code/test cases. The exact package command has the same non-owned pipeline compile blocker as G1.
- [ ] G3: Templates compile under the production strict compiler; incomplete or
  ambiguous examples fail.
  CHECK: go test ./tests -run 'TestSpecSkillTemplate|TestSpecParser.*Quoted|TestSpecCompiler' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: OPEN (2026-08-27) — `TestSpecSkillTemplate` passes directly, including gap/constraint/prose/open-outcome negatives. The exact compiler/parser command is temporarily compile-blocked by the same non-owned pipeline migration as G1; current compiler/parser sources are no longer the blocker.
- [ ] G4: Skill/docs contain no finite-adapter/generated-verifier requirement
  and never describe tests, mutation, PICT, or sampling as mathematical proof.
  CHECK: ! rg -n -i 'finite-adapter|ray-adapter-v1|generated verifier|mutation.*(proof|proves)|PICT.*(proof|proves)|sampling.*(proof|proves)' skills/spec skills/task
  EXPECT: /^$/
  EVIDENCE: PASS (2026-08-27) — exact command exited 0 with empty output. The skills retain actual public/hidden verifier translation and describe coverage/PICT, mutation, fuzzing, property-based testing, dependency harvesting, and differential samples only as witness-finding/supporting layers that cannot issue `VERIFIED` or replace the four exact checks.
