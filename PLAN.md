# Plan: finish Ray v0.10

Depth: tree 4  
Mode: orchestrated

## Fixed product definition

The only authoritative design files are:

- `docs/specs/finalarchitecture.md`
- `docs/specs/whole flow.md`
- `docs/specs/architecture-freeze.sha256`

Their hashes must continue to verify before any implementation result is
accepted. The rejected drafts and the dated v0.1 design are historical only.

Ray evaluates one frozen, fixed, bounded coding task or PR. `spec.md` is its
strict machine-readable description of the complete finite behavior. It is
compiled into Spec Semantic IR; proof stages never infer requirements from
tests or copy expected results into independently translated code/test IR.

For the complete finite input/state set `D`, complete observable outcome set
`O`, required behavior `R`, exact reference behavior `C`, complete
implementation behavior `F : D -> O`, and exact frozen verifier predicate
`T(F)`, production must establish all four results:

```text
Reference correctness:
  EXISTS x,o: C(x,o) AND NOT R(x,o)                         = UNSAT

False-positive freedom:
  EXISTS F: T(F) AND EXISTS x: NOT R(x,F(x))                = UNSAT

False-negative freedom:
  EXISTS F: (FORALL x: R(x,F(x))) AND NOT T(F)              = UNSAT

Exact reference acceptance:
  T(C)                                                       = true
```

The tests are one global predicate over the entire behavior vector. The model
must preserve cross-case, stateful, ordering, effect, and shared-state
relationships declared inside the bounded task scope. Enumeration,
SAT/SMT/BDD, or compiler/model-checker frontends may be used only when exact.
Sampling, PICT, mutation, fuzzing, property-based testing, names, tokens, or
coverage percentages can find counterexamples but can never produce
`VERIFIED`.

Python, Rust, and C++ frontends independently translate the real reference and
real verifier. Unsupported reachable semantics, incomplete translation,
unfrozen evidence, stale hashes, solver `UNKNOWN`, or skipped stages produce
`PROOF BLOCKED`.

The current adapter/generated-verifier branch is rejected architecture. Remove
it from production and tests. Ray must translate the actual reference and
actual verifier described above; it must not replace the verifier with one
generated from the spec, because doing that would erase the false-positive and
false-negative questions.

Existing Ray layers remain:

```text
spec-lint -> coverage/PICT -> oracle -> diff-test -> dep-harvest
          -> exact Semantic-IR proofs -> real counterexample confirmation
```

Only the exact Semantic-IR proofs issue the mathematical all-clear.
PICT, simplified-oracle proof, and diff-test run for every positive task;
`dep-harvest` may record typed not-applicable evidence only when the frozen task
declares no relevant dependency. A mandatory stage that is absent, skipped, or
unknown blocks `VERIFIED` even though a clean diagnostic result is not itself a
proof.

## Shared implementation contracts

- `internal/semanticir` owns canonical typed Spec, reference, Test, and
  environment representations, provenance, translation completeness, digests,
  and proof evidence.
- `internal/speccompiler` is the only proof-path reader of `spec.md`.
- `internal/frontend/{python,rust,cpp}` translates actual frozen code and test
  semantics independently and fails closed on unsupported reachable behavior.
- `internal/testir` builds one exact global `T(F)` from the real frozen verifier
  and pass signal. No test-name matching or per-row union is authoritative.
- `internal/proof` decides the three UNSAT queries and `T(C)` over the complete
  bounded model and returns exact witnesses or blockers.
- `internal/executor` confirms witnesses in fresh disposable copies of the real
  frozen environment and restores originals byte-for-byte.
- `internal/taskbundle` and `internal/certificate` bind all artifacts, tools,
  environment, Semantic IR, proofs, and confirmations by digest.
- `internal/pipeline` plus `cmd/ray` runs one fail-closed path for `ray start`
  and `ray check`.
- Mutation and other diagnostic layers cannot affect the formal verdict.

No leaf may add a second architecture, task-specific production path, authored
mutation recipe, generated replacement verifier, or result copied from the
spec into code/test translation.

## Work tree

- 1. Ray v0.10
  - 1.1 Semantic foundation
    - 1.1.1 Semantic IR and strict spec compiler (`gates/leaf-ir.md`)
    - 1.1.2 Task freeze and certificates (`gates/leaf-freeze-cert.md`)
    - 1.1.3 Spec authoring skill (`gates/leaf-spec-skill.md`)
  - 1.2 Independent translation
    - 1.2.1 Python frontend (`gates/leaf-python.md`)
    - 1.2.2 Rust frontend (`gates/leaf-rust.md`)
    - 1.2.3 C++ frontend (`gates/leaf-cpp.md`)
    - 1.2.4 Exact global Test IR (`gates/leaf-testir-exhaustive.md`)
  - 1.3 Formal verification
    - 1.3.1 Exact proof engine (`gates/leaf-proof.md`)
    - 1.3.2 Real witness confirmation (`gates/leaf-executor.md`)
  - 1.4 Product integration
    - 1.4.1 Pipeline and CLI (`gates/leaf-pipeline.md`)
    - 1.4.2 Python/Rust/C++ end-to-end tasks (`gates/leaf-e2e.md`)

## Integration order

1. Remove the rejected adapter/generated-verifier architecture and restore a
   compiling shared IR.
2. Finish Spec IR and exact global Test IR contracts.
3. Finish independent Python, Rust, and C++ reference/test translation.
4. Finish the three UNSAT proofs and exact-reference acceptance.
5. Finish witness execution, artifact freezing, and certificates.
6. Wire the same path through `ray start` and `ray check`.
7. Run real positive and adversarial tasks in all three languages.
8. Rerun every root gate, race checks where practical, and `go vet ./...`.

## Status log

- 2026-08-27: Earlier broad adapter/generated-verifier architecture rejected.
- 2026-08-27: Final architecture and whole flow rewritten to the user's exact
  false-positive/false-negative/reference objective.
- 2026-08-27: Final documents hash-frozen read-only; both SHA-256 checks pass.
- 2026-08-27: First full repository run compiled but failed across Python,
  certificates, proof, CLI, pipeline, and E2E because rejected adapter closure
  requirements still control production.
