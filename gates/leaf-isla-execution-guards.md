# Gates: shared instruction-visit guard

Scope: Apply the existing native guard to both stages without discarded paths or partial verdicts.

- [x] G1: Both native tools accept a positive visit limit. Invalid limits fail explicitly.
  EVIDENCE: Both binary targets passed pc_visit_limit tests for absent, positive, zero, negative, malformed, and overflowing values. Direct calls to both release tools rejected zero and malformed values with exit 1 and Invalid --pc-limit errors. Real SDK results passed with a positive limit. The previous trace tool returned exit 1 and Unrecognized option: 'pc-limit'. No discard mode was added to the trace tool.

- [x] G2: Public semantic evidence records the request limit. Mismatched evidence returns no verdict.
  EVIDENCE: TestExecutionStagesReceiveVisitLimit, TestSemanticEvidenceRetainsVisitLimit, and TestMatchingRejectsVisitLimitDifferences passed in the all-package Go command. They assert the exact request argument, retained evidence, and rejection of each changed limit. External real-test callers assert both public evidence values against their request limit.

- [x] G3: The existing recursive executable exhausts the trace guard with no verdict. Sufficient limits retain both expected results.
  EVIDENCE: TestRealRustRecursionRejectsVisitLimit passed in 37.87 seconds. The public SDK returned the trace-stage PCLimitReached error without a verdict. The first assertion expected the former solver display text and failed. The corrected assertion requires the actual native trace error. TestRealRustPatterns/recursive_calls passed both the correct result 6 and changed-requirement counterexample queries with four visits. Both public evidence records equal that request limit.

- [x] G4: Native tests, Go tests, real-tool regressions, style measurements, and source reconstruction pass.
  EVIDENCE: 158 native tests passed. All Go packages, race tests, and integration-tagged vet passed. All 24 listed real-tool cases have passing results across the full command, corrected recursion rerun, and new repair test. The full command retained the obsolete solver-text assertion and has one recorded failure, which the corrected rerun resolves. New modules have no selected Clippy diagnostics. Changed-file formatters and measured Go function/file limits passed. Inherited upstream warnings remain. All 73 exported source files matched a patched clean checkout. measured-execution-guards.json records the identities and command outcomes.
