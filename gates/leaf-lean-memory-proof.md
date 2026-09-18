# Gates: imported Sail memory proofs

Scope: Prove reusable memory identities from the imported support definitions.
These gates do not close the full translation or bounded-program objective.

- [x] G1: Lean accepts the entire pinned generated RV64 model.
  EVIDENCE: Session 84829 returned exit 0: Build completed successfully (135 jobs).

- [x] G2: Lean proves read-after-write for every address, byte, and prior state.
  EVIDENCE: Parent session 98772 replayed ReadAfterWrite.lean with warningAsError=true and returned exit 0.

- [x] G3: Lean proves that a byte write preserves reads at distinct addresses.
  EVIDENCE: Parent session 98772 replayed DistinctAddress.lean. Its theorem preserves both value and error results, including the resulting memory state.

- [x] G4: Lean proves that absent-memory reads retain the upstream error.
  EVIDENCE: Parent session 98772 replayed AbsentRead.lean. The theorem returns OutOfMemoryRange with the original state.

- [x] G5: The proofs import upstream operations and contain no assumed memory laws.
  EVIDENCE: All three files import Sail.ConcurrencyInterfaceV1. Their guarded axiom reports contain only propext, Classical.choice, and Quot.sound. The source scan found no sorry, axiom, native_decide, opaque, or replacement def.

- [x] G6: The replay pins dependencies and guards proof axioms and exact scope.
  CHECK: bash proof/sail_memory/shell/run.sh /Volumes/Hak_SSD/hyperray-research/sail-riscv-lean-proof-audit /tmp/lean429-local/lean-4.29.0-darwin_aarch64/bin/lake
  EXPECT: Imported memory proof replay passed.
  EVIDENCE: Parent session 98772 returned exit 0. The replay requires clean exact model/support revisions and Lean 4.29.0. An invalid Lake executable returned exit 1 with Lean release differs from 4.29.0.

- [x] G7: The parent replays each proof and rejects changed or incomplete evidence.
  EVIDENCE: Parent session 9347 returned exit 0 after all three proofs and Changed memory claim rejected. Session 66883 operated a temporary copy of the same replay without proof files and returned exit 1: missing required proof: AbsentRead. The valid files remained unchanged. bash -n and git diff --check passed. The replay has 58 lines.
