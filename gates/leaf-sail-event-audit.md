# Gates: Sail event-interface audit

Scope: Establish reusable event properties and reproduce the RISC-V integration result.

- [x] G1: The unmodified pinned RISC-V generation attempt has retained evidence.
  EVIDENCE: generation.sh exited 0 and printed "Control generated; event interface mismatch reproduced." Records are /tmp/hyperray-event-generation.vDNAyP. Control generation produced Lean_Control/LeanControl.lean. events.log reports "Unknown outcome variable 'pa in instantiation". compiler.txt and digests.txt retain the tool and configuration identity. The generated Lean model was not compiled in this audit.

- [x] G2: Lean accepts generic finite-choice and instruction-effect preservation proofs.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/sail-riscv-lean-proof-audit && /tmp/lean429-local/lean-4.29.0-darwin_aarch64/bin/lake env lean -DwarningAsError=true /Volumes/Hak_SSD/hyperray/proof/sail_events/lean/Preservation.lean && echo EVENT_IDENTITIES_PASSED
  EXPECT: EVENT_IDENTITIES_PASSED
  EVIDENCE: EVENT_IDENTITIES_PASSED

- [x] G3: The audit reproduces primitive coverage mismatches and rejects false claims.
  CHECK: bash /Volumes/Hak_SSD/hyperray/proof/sail_events/shell/replay.sh /Volumes/Hak_SSD/hyperray-research/sail-riscv-lean-proof-audit /tmp/lean429-local/lean-4.29.0-darwin_aarch64/bin/lake && echo EVENT_AUDIT_PASSED
  EXPECT: EVENT_AUDIT_PASSED
  EVIDENCE: Imported event audit passed; full coverage remains open. | EVENT_AUDIT_PASSED

- [x] G4: All audit sources satisfy the code-style limits and the replay succeeds.
  CHECK: cd /Volumes/Hak_SSD/hyperray && bash -n proof/sail_events/shell/replay.sh proof/sail_events/shell/generation.sh && awk 'FNR > 75 { exit 1 }' proof/sail_events/lean/*.lean proof/sail_events/shell/*.sh fixtures/lean/changed-barrier.lean fixtures/lean/changed-range.lean && echo EVENT_SOURCE_LIMITS_PASSED
  EXPECT: EVENT_SOURCE_LIMITS_PASSED
  EVIDENCE: EVENT_SOURCE_LIMITS_PASSED. Six new code files contain at most 59 lines. The longest function or theorem spans 7 lines (Preservation.lean:17-23). Meaningful nesting does not exceed two levels. Bash syntax checks and the guarded Lean replay passed. No trailing whitespace or admitted proof occurs.
