# Gates: solver-backed model-call safety

Scope: Connect existing model-call events to the public bounded safety result.

- [x] G1: The public boundary binds sorted call names and rejects malformed input without changing caller data.
  EVIDENCE: TestProgramBindsForbiddenCalls passed with a changed query digest, unchanged ELF digest, sorted artifact data, preserved caller order, and an explicit size-limit error. TestProgramRejectsInvalidForbiddenCalls rejected empty names, malformed names, duplicate names, and a non-sequential profile. go-all-final.log retains the package results.

- [x] G2: Native validation rejects unknown functions and missing instrumentation. Candidate constraints reject infeasible fault witnesses.
  EVIDENCE: All five forbidden_calls native tests passed. The tests cover malformed arrays, absent fields, actual function resolution, non-functions, absent instrumentation, return-only events, unrelated calls, and candidate isolation. The real solver passed all four requirement/call truth-table rows and returned UNSAT for contradictory path constraints with a forbidden call. native-focused.log and native-workspace.log retain the results.

- [x] G3: Both tools require the capability. Real compiler programs expose normal and trap outcomes through the public SDK.
  EVIDENCE: TestForbiddenCallsRequireBothCapabilities and TestExecutableRejectsMissingCallCapability passed. Missing support in either whole-program tool returned no verdict. The real suite passed TestRealRustForbiddenTrapCall in 14.28 seconds and TestRealRustNormalCallSafety in 33.62 seconds. Their states contain called:trap_handler=true and called:trap_handler=false respectively. Both tests also assert the public ModelCalls map. Protocol tests reject absent, duplicate, unknown, and non-Boolean observations. All logs are in /tmp/hyperray-forbidden-calls.sqR4AZ/.

- [x] G4: Regression tests and changed-source style measurements pass. Existing release artifacts and the Git index remain unchanged.
  EVIDENCE: All twenty listed real-tool tests passed in one command in 478.830 seconds. The name comparison found no missing, failed, or skipped tests. The native workspace passed 148 tests without failed or ignored tests. All Go packages, integration-tagged go vet, and the Isla race test passed. The race test took 8.694 seconds. New native modules have no Clippy diagnostic under the configured shape limits. The new native files have at most 69 lines, and changed Go functions have at most 31 lines. Existing upstream warnings remain. The source patch applied to a clean checkout, and all 60 exported files matched. The original Git index and earlier release artifacts retain their recorded digests. /tmp/hyperray-forbidden-calls.sqR4AZ/ retains the logs.
