# Symbolic instruction addresses

Status: the five call and storage fixtures pass. General address coverage remains open.

## Measured problem

The generic-call, dynamic-call, and recursive-call fixtures all stopped with
`instruction PC is not concrete` after the sequential-memory integration.
The local stack-array and stack-value fixtures passed their two proof queries.
The fixtures and engine source remained unchanged during these measurements.

`machine/isla/semantic_instruction.go` accepts only hexadecimal literals in
program-counter read events. It rejects a symbolic value before coverage acceptance.
The investigation must retain the real event tree and its path constraints.
No program-specific address rule can replace this evidence.

The captured generic-call trace has a symbolic PC after its last instruction.
Its final read fetches the return-boundary bytes but emits no instruction event.
The current parser rejects this unused PC immediately.
The first repair defers the concrete-address requirement until an instruction event.
A symbolic PC clears an earlier concrete PC. Sibling branches retain separate state.
Malformed register events still return errors, including malformed terminal events.
This repair does not resolve a symbolic address that an instruction actually uses.

The first repeat passed the Hyperray parser but exposed the same boundary order in Isla.
`INSTR_ANNOUNCE` applies the PC-visit limit before its existing zero-announcement exit.
The dump omits that boundary from instruction events, but the bounded executor counts it first.
The existing explicit exit must occur before the instruction counter.
This change does not add an exit condition or alter ordinary instruction counting.

After both repairs, generic calls and dynamic dispatch passed their positive and changed-result queries.
Recursion still contains a symbolic address on an executed instruction.
The logs are in `/tmp/hyperray-sequential-replay.441HmB/`.

The existing `Memory::check_concrete_overlap` supplies a reuse candidate.
It obtains a satisfiable address and asks whether a different address is possible.
It accepts uniqueness only after the second query is unsatisfiable.
It returns an error for an unknown solver result.
The address connection must preserve these safeguards without adding instruction-specific rules.

## Shared value resolution

The memory-overlap check and sequential memory reads will share the existing uniqueness mechanism.
A sequential read can return a concrete value only after a satisfiable query and an unsatisfiable different-value query.
The original symbolic read event and its array constraint remain in the trace.
The callback changes only the returned representation, not the permitted values.
The default memory profile retains its current return representation.

An unconstrained or multi-valued read remains symbolic.
A value wider than the concrete bitvector representation also remains symbolic.
Unknown solver results and missing models remain explicit errors.
An unsatisfiable query cannot establish a unique value.
The uniqueness query must not add a persistent assumption.

For current trace constraints `C`, read value `v`, and candidate `c`, the two queries are
`SAT(C)` and `UNSAT(C AND v != c)`.
Together, these results establish that every permitted value of `v` equals `c`.
`Solver::check_sat_with` uses the upstream assumption-query interface without a persistent assertion.
The memory event still contains `v`, so trace replay retains the original byte-array equality.
This argument covers the replacement condition. It is not a proof of the entire translator.

Native tests must cover unique values, multiple values, inconsistent constraints,
symbolic widths, and retention of both alternatives after the query.
Public memory tests must cover stored pointers, changing stored values, and untouched symbolic memory.
The real recursion fixture and the other call fixtures must remain unchanged.

## Independent execution limits

The first real recursion replay passed the symbolic-PC step but exceeded the visit limit.
The fixture request used the host-worker count as the instruction-visit limit.
These limits describe different resources and must have separate names.
The recursive fixture masks its input with `3` and calls depths `3`, `2`, `1`, and `0`.
Its declared instruction-visit limit is four, independent of the two host workers.
The fixture source and result requirements remain unchanged.
A second request with a visit limit of two must return an error without a proof result.

## Native regression evidence

The rebuilt `isla-lib` passed 99 unit tests and three documentation tests.
`/tmp/hyperray-unique-read.uazx9M/native-final.log` retains the output.
The added tests cover unique values, retained alternatives, inconsistent constraints,
concrete-region overlap, stored pointers, default memory behavior, and trace replay.
The unknown-result test exercises the existing error conversion, not an actual solver timeout.
The complete upstream workspace test command also passed.
`/tmp/hyperray-unique-read.uazx9M/workspace.log` retains its output.

Rustfmt passed for the changed small Rust files.
Clippy reported no diagnostic in the unique-value or sequential-memory modules.
The rest of the upstream code still has diagnostics. The dependency is not warning-free.
The Go package race tests passed in 10.957 seconds. Integration-tagged `go vet` passed.

## Real program results

All five cases in `TestRealRustPatterns` passed their declared-result and changed-result queries.
The program sources and required results remained unchanged.
Recursion passed after the shared read resolution and separate visit-limit declaration.
The two-visit recursion request returned an error without a verdict.
Invalid stack access also retained an error without a verdict.
Mixed-width memory and symbolic branch-input regressions passed.
The logs are `patterns.log` and `regressions.log` in `/tmp/hyperray-unique-read.uazx9M/`.
The remaining real-tool tests passed in `integration-rest.log`.
A name-by-name comparison with `go test -list` found fourteen listed tests and fourteen passing tests.
No listed test was missing, failed, or skipped.
The compiler-built Rust fixture ledger has four of four completed gates.
The later source reconstruction passed the separate gates in `gates/leaf-isla-reproducible-source.md`.

These results cover the declared fixtures. They do not establish general multiple-target coverage,
full translation equality, concurrent memory semantics, or the 100 MB product target.

## Required repair

### Several permitted targets

The next regression uses a compiler-built table with two function pointers.
A symbolic input selects the table index. Both functions must retain reachable entry events.
The declared outputs are `17` and `64`. Every other output must be impossible.
Separate queries must return counterexamples for each permitted output.
The compiler evidence must retain the indirect call and both target functions.

The reuse candidate is upstream `Instr::Monomorphize` in `isla-lib/src/executor.rs`.
The source and `doc/axiomatic.adoc` describe solver-backed value splitting.
One branch asserts the selected value. A queued branch excludes that value and continues the same operation.
The queued branch ends only after its remaining constraints become unsatisfiable.
An unknown solver result returns an error.
The integration must retain this partition and must not use only its first model value.
The retained function-table trace supplies evidence for this connection point.

The baseline function-table test exceeded the semantic deadline in 149.74 seconds.
The retained diagnostic reached a symbolic table read and then an instruction fetch through its result.
The compiled Sail model already has the `__monomorphize_reads` control.
It calls the existing address-partition operation before the memory primitive.
No instruction-specific translation is necessary to activate that operation.

The control must apply to program execution, not isolated instruction analysis.
A global configuration experiment exceeded the footprint deadline in 127.71 seconds.
Isolated load instructions have unconstrained addresses, so full address enumeration is inappropriate there.
Both tools already accept separate `--footprint-config` input.
The execution experiment must use `-I` for the initial control value.
The `-R` reset option is insufficient because this model calls that reset hook only in its footprint entry.
Logs and the retained program are in `/tmp/hyperray-multiple-targets.IMhJIb/`.

The corrected raw dump completed with `-I __monomorphize_reads = true` and the original footprint configuration.
Hyperray supplies these same options to both sequential program stages.
The independent footprint request retains its existing arguments.
The axiomatic profile also retains its existing arguments.
The public function-table test passed all three queries in 124.88 seconds.
Its first query rejects the complement of the complete declared output set, not only one incorrect value.
Both target functions retained entry events. Each permitted output had a concrete counterexample.
All fifteen listed real-tool tests passed in the same command, with no missing, failed, or skipped tests.
The command completed in 886.794 seconds.
The clean rebuilt tools also passed the function-table test in 126.57 seconds.
The logs are `full-integration.log` in `/tmp/hyperray-multiple-targets.IMhJIb/`
and `clean-integration.log` in `/tmp/hyperray-isla-source.oWNCM0/`.

An instruction address must follow from the same trace and its constraints.
A solver model alone does not establish that an address is unique.
The integration must retain every permitted address or return an explicit error.
It must not select one model value and discard the other values.
The existing upstream event and solver interfaces are the first reuse candidates.

The repaired integration must pass the unchanged call fixtures and their changed requirements.
It must preserve multiple feasible targets and reject missing or conflicting address evidence.
An unreachable trace must not supply vacuous address evidence.
Full translation equality and full program coverage remain separate requirements.
