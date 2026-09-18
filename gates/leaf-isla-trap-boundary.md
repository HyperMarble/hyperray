# Gates: observable trap behavior

Scope: Replace the missing trap callback with a reusable, explicit fault outcome without losing executions.

- [x] G1: The callback contract comes from source that matches the pinned signature.
  EVIDENCE: The compiled snapshot declares ztrap_callback as unit to unit at line 12672. Its trap_handler calls the callback at line 34228 before architectural updates. Local upstream commit b6f7b1df64157e6b9d250e552842f963394bc2ba defines the same callback signature with function trap_callback(_) = () in model/riscv_callbacks.sail. The current source has a different signature and is not substituted.

- [x] G2: A retained diagnostic preserves the fault path after the existing notification mechanism.
  EVIDENCE: The first diagnostic retained handler-address dependence and ended at the 120-second deadline with exit 124 and no completed trace. With an explicit trap-entry stop boundary, the second diagnostic exited zero and produced a 42,574-byte trace. It records mcause 7, mtval 0xfffffffffffffff8, mepc 0x80100020, and nextPC 0x80100048 before the boundary. The original executable bytes and production configuration remain unchanged. /tmp/hyperray-trap-boundary.BBILdt/ retains both diagnostics. This is fault-state trace evidence, not a proof verdict.

- [ ] G3: The public result exposes modeled trap evidence under an explicit bounded handler contract.
  EVIDENCE: Partial. The public SDK accepts typed mtvec and medeleg values and exposes solver-selected trap_handler calls. The register-field leaf also passed all four gates. Its public cause query proves mcause.bits = 7, rejects a changed requirement, and retains that cause with the forbidden trap witness. The complete exit-reason contract remains open. The newer evidence is in /tmp/hyperray-register-fields.JnFBoC/real-formatted.log.

- [ ] G4: Valid programs, invalid pointers, changed requirements, and missing handler contracts retain correct outcomes.
  EVIDENCE: Partial. All 23 listed real-tool tests passed with the register-field tools, including valid programs, changed requirements, typed fault state, structured cause assertions, and the original missing-callback error without a verdict. The full handler-contract rejection matrix remains open.
