# SDK prototype: end-to-end test pass

Status: The buggy-code-to-repair test passed. All 24 listed real-tool cases have passing results across the recorded commands.

## Tested path

The test driver compiles Rust source with the actual compiler and linker.
It supplies the resulting executable, a requirement, and explicit limits through the public SDK.
The SDK prepares the program input, obtains instruction traces, and requests the whole-program model and solver result.
The caller receives the verdict, counterexample values, artifact identities, and execution evidence.
An unsuccessful stage returns an error without a verdict.

This is an integrated SDK test, not a comparison of isolated program outputs.
The tests do not replace the compiler, model, or solver with fake tools.
Separate protocol tests use fake tools, but they do not count as real-tool evidence.

The public entry points are `BuildProgram`, `NewRequest`,
`NewVerificationRequest`, and `ExecutableVerifier.VerifyProgram`.
The external test package constructs these values without access to private SDK fields.

## Required outcomes

| Case | Required public result |
|---|---|
| Declared result of the compiler-built program | `PROVED` |
| Changed requirement for the same program | `DISPROVED` and concrete counterexample values |
| Buggy stack update, followed by its repair under the same requirement | `DISPROVED` with result 12, then `PROVED` for required result 13 |
| Forbidden trap under the declared fault boundary | `DISPROVED` with the feasible trap call |
| Structured fault-cause requirement | A proof or cause counterexample, according to the stated requirement |
| Exhausted instruction-visit limit | Trace-stage error, no verdict |
| Missing model field or invalid typed state | Error, no verdict |

## Scope

The driver supplies the requirement and limits. This pass does not demonstrate automatic generation of requirements from an agent prompt.
The tested route uses the pinned RV64 model and the existing compiler fixtures.
These tests do not establish full language coverage, translation equivalence, or Anchor integration.

The eventual 100 MB target includes Hyperray and all child tools.
The eventual Anchor target is 200–300 MB for its full process group.
The current tests use a 2048 MB model-tool limit where specified.
That limit is not a measurement of memory consumption.
Memory optimization follows this functional test pass.

The latest evidence directory is `/tmp/hyperray-execution-guards.JlGvKA/`.
`real-all.log` retains the full regression command output.
`recursion-corrected.log` retains the corrected visit-limit test.

## Measured repair result

`TestRealRustStackRepairKeepsRequirement` passed in 111.14 seconds.
The driver compiled `stack_array_bug.rs`, which adds eight to the selected array element.
The unchanged requirement demands result 13 for input 2.
The SDK returned `DISPROVED` and `0:x10=#x000000000000000c;`, which is result 12.

The driver then compiled the existing `stack_array.rs`, which adds nine.
The same requirement returned `PROVED`.
The test also requires different program identities and matching instruction-visit guards in the public evidence.
`repair.log` retains both compiler calls and the measured results.

The test driver selects the known repair. This result does not demonstrate an autonomous agent that reads the counterexample and writes a repair.

## Regression accounting

The full command completed in 1141.383 seconds with 22 passes and one failed assertion.
That assertion expected the previous solver error text. The trace stage correctly returned `PCLimitReached` without a verdict.
The corrected recursion test passed in its separate 38.335-second command.
The added repair test passed in a separate 111.693-second command.

The name-by-name audit found 24 listed tests and 24 distinct passing results.
No listed test lacks a passing result. No test was skipped.
This is not a claim that the original full command exited successfully.
The native workspace also passed 158 tests. All Go packages, race tests, and integration-tagged vet passed.

`tools/isla/measured-execution-guards.json` records source and executable identities.
