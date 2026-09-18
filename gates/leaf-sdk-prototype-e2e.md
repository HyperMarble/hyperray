# Gates: SDK prototype end to end

Scope: Compile a buggy implementation and its repair, then evaluate both through the public SDK under the same requirement.

- [x] G1: The actual compiler and linker build both source files. The SDK generates their program inputs without program-specific production rules.
  EVIDENCE: TestRealRustStackRepairKeepsRequirement compiled stack_array_bug.rs and stack_array.rs with rustc 1.98.0 and LLD 22.1.8. Both binaries entered BuildProgram and ExecutableVerifier.VerifyProgram through package isla_test. No production code changed for this fixture. /tmp/hyperray-execution-guards.JlGvKA/repair.log records both compiler calls and the final pass.

- [x] G2: The buggy implementation returns a concrete counterexample. The repaired implementation returns a proof under the unchanged requirement and logical input.
  EVIDENCE: The 111.14-second test passed. Both programs use input 2, required result 13, sequential memory, and the same visit guard. The buggy code adds 8 and returns counterexample 0:x10=#x000000000000000c;. The repaired code adds 9 and returns PROVED. The test asserts an unchanged negated assertion and different program identities.

- [x] G3: Every listed real-tool test has a passing result on the latest native build. Required errors return no verdict.
  EVIDENCE: The test-list audit found 24 listed names and 24 distinct passing results, with none missing or skipped. real-all.log contains 22 passes and the obsolete recursion error-text assertion failure. recursion-corrected.log supplies that corrected pass. repair.log supplies the additional same-requirement repair pass. All three commands used the execution-guards-v1 binaries. Required error cases assert no verdict.

- [x] G4: The report identifies the tested path and keeps full coverage, Anchor integration, and total-process memory optimization outside this measured milestone.
  EVIDENCE: docs/prototype-end-to-end.md names the external SDK API, actual compiler route, requirement ownership, and pending regression command. It states that the 100 MB target includes all child tools and remains unmeasured. Full coverage and Anchor integration are not claimed.
