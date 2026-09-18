# Gates: typed initial machine state

Scope: Carry model-typed initial state through the public program API and both real execution stages.

- [x] G1: Public construction preserves state in the program digest and rejects ambiguous declarations.
  EVIDENCE: TestProgramPreservesTypedInitialState passed with a changed program digest and unchanged image digest. TestProgramRejectsInvalidInitialState rejected overlap with integer initialization, duplicate typed names, invalid names, empty values, and multiline values. Both tests use the public BuildProgram API.
  The canonical-order test preserved both the digest and the caller's input
  order. The size-limit test returned ResourceLimit without a partial program.

- [x] G2: The reused parser and model declarations reject invalid names, shapes, widths, and element types.
  EVIDENCE: All three initial_state native tests passed. The declared model fixture accepts five typed assignments. Thirteen rejected assignments cover unknown names, non-register names, wrong widths, missing and extra fields, wrong field and element types, vector length, enumeration membership, and malformed syntax. The public TestRealRustRejectsWrongTypedState passed in 32.21 seconds and returned the mtvec type error with no verdict. /tmp/hyperray-initial-state-native.log and /tmp/hyperray-initial-state.BtoOF9/public.log retain the results.

- [x] G3: Both execution stages require the capability while standalone footprint analysis remains unchanged.
  EVIDENCE: TestTypedInitialStateRequiresBothCapabilities and TestFootprintExcludesTypedInitialState passed. TestExecutableRejectsMissingTypedStateCapability passed for both a legacy solver and a legacy semantic tool. Both cases returned the missing-capability error without a result.

- [x] G4: A compiler-built fault case passes both queries through the public SDK with typed trap state.
  EVIDENCE: TestRealRustTypedTrapState passed in 67.17 seconds with the newly built tools. The compiler retained the declared stack adjustment and store offset. The fault-address requirement returned PROVED. Its changed requirement returned DISPROVED with 0:mtval=#xfffffffffffffff8;. The public program supplies typed mtvec and medeleg state. The fixture configuration supplies only the matching unit notification callback. /tmp/hyperray-initial-state.BtoOF9/public.log retains the result.

- [x] G5: Native tests, existing Go tests, real regressions, and changed-source style checks pass.
  EVIDENCE: All eighteen listed real-tool tests passed across all-real.log and typed-input-final.log, with no missing, failed, or skipped tests. The native workspace passed 143 tests. The final Go package suite passed, and the final Isla race suite passed in 7.717 seconds. Integration-tagged go vet and format checks passed. Clippy with the configured shape limits reported no diagnostic in the new native modules. New native files have at most 71 lines, and new Go functions have at most 31 lines. Existing upstream warnings remain outside this leaf's new-code checks. The source patch applied cleanly, all 54 exported files matched the working source, and all ten recorded file digests matched. /tmp/hyperray-initial-state.BtoOF9/ retains the logs.
