# Solver-backed model-call safety

The program boundary can name forbidden model functions in `ForbiddenModelCalls`.
These names describe model behavior, not source-language patterns.
For example, a profile can forbid entry into the model's trap handler.
This field adds a safety requirement to the existing final-state requirement.

The generated program includes a sorted `forbidden_model_calls` array.
The program digest covers that array. Both whole-program tools receive
`--forbidden-model-calls` and the existing `--trace-function` option for each name.
The isolated instruction analysis retains its original configuration.

The native integration resolves each name against actual model functions.
Unknown names, non-functions, missing instrumentation, duplicates, and a
non-sequential memory profile return errors before program execution.
An empty list preserves existing behavior.

For each candidate, the existing function-call events supply a Boolean value.
Return events do not count as calls. Events in other candidates cannot affect it.
The candidate query is:

```
existing path constraints AND (negated final requirement OR forbidden call)
```

The solver still receives every existing path constraint.
A forbidden call on an inconsistent path cannot produce a counterexample.
No path disappears because it contains a forbidden call.
Each allowed candidate includes `called:<model-function>=true;` or `false;`
in its concrete state. The public counterexample retains these values in
`CounterexampleState` and the `ModelCalls` map.
Missing, duplicate, unknown, or non-Boolean call observations return a protocol error.
These values describe the same candidate that the solver accepts.

This feature does not supply an operating-system handler or a missing primitive.
The caller still declares the finite handler boundary and initial machine state.
An incomplete execution, missing model behavior, or solver error yields no verdict.
Structured final-register observations and full translation equivalence remain open.

## Measured results

The public fault test returned `DISPROVED` with `called:trap_handler=true;`.
The normal-return test proved its declared return value without a trap.
Its changed requirement returned `DISPROVED` with the return value and `called:trap_handler=false;`.
Both tests also asserted the public `ModelCalls` map.
The trap-entry boundary came from the compiled ELF, not a fixed production address.

All twenty listed real-tool tests passed in one command in 478.830 seconds.
The native workspace passed 148 tests without failed or ignored tests.
The new native solver tests rejected a contradictory fault path.
All Go packages, the race test, and integration-tagged `go vet` passed.
Changed-source format checks passed. Existing upstream warnings remain.

The cumulative source patch reconstructs 60 files from the pinned upstream commit.
All 60 files matched the working source. Earlier release artifacts remain unchanged.
`tools/isla/measured-forbidden-model-calls.json` records the new local build.
The evidence directory is `/tmp/hyperray-forbidden-calls.sqR4AZ/`.
