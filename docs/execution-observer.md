# Experimental execution observer

## Contract

The public `execution.Run` function launches a precompiled checker.
The `hyperray observe REQUEST.json` command calls the same function.
The caller supplies an absolute executable path, arguments, working directory,
environment, timeout, output limit, and memory budget.
The observer does not compile code or interpret instructions.

This path is separate from the production proof stages and their evidence rule.
An exit code of zero means only that the worker exited successfully.
It never establishes `PROVED`, complete search, or semantic coverage.
A backend-specific validator must establish those separate claims.
The existing native SPIN and Loom experiments supply the first integration cases.
Their documented limits remain unchanged.

## Results and limits

The result contains the worker exit code, combined output, elapsed time,
worker peak memory, observer peak memory, and their sum.
The observer reports timeout, cancellation, output overflow, and memory excess.
It retains partial output and never silently accepts truncated output.
Start errors and unavailable measurements return errors, not execution results.

The initial platform is macOS. Its `getrusage(2)` manual specifies bytes.
Other platforms return an explicit error before execution.
The runtime budget cannot exceed 100,000,000 bytes, as the user requested.
Memory acceptance requires a measured sum strictly less than the supplied budget.
This is a measurement gate, not a hard operating-system memory cap.
The observer peak includes earlier allocations in the same host process.
Compilation and later JSON output are outside the measured worker interval.

This observer supports a trusted single-process checker, including its threads.
It does not measure descendant processes or provide a security sandbox.
The caller must not supply an untrusted program or a worker that creates processes.
Cancellation targets the worker process group. This is cleanup, not isolation.
The request supplies the exact environment. An empty array means no inherited
environment variables. A missing environment is an input error.
The observer does not establish compiler, binary, or dependency provenance.

## Acceptance

External-package tests exercise the public API without access to private fields.
Local process fixtures exercise ordinary exits, nonzero exits, both output
streams, cancellation, timeout, output limits, and invalid requests.
The integration matrix operates all twelve corrected native checkers, their
stored-pointer counterexample and replay, and all six paired Loom checkers.
These results establish execution integration only, not a completed proof SDK.

The implementation uses Go's local `os/exec` source contract for combined output,
cancellation, and bounded pipe cleanup. It uses the local macOS memory manual.
The gate ledger is `gates/leaf-execution-observer.md`.

## Measured result: 2026-09-05

All twenty integration cases passed through the public SDK entry point.
The final matrix reported a peak sum of 33,062,912 bytes, or 33.06 MB.
This sum includes the compiled checker and its Go observer, not the Go compiler.
The corrected cases retained their completion output. The broken cases retained
their counterexample or assertion output. The stored-pointer replay also passed.

The Go regression suite, `go vet`, and the observer race checks passed.
The Linux test binary compiled. No Linux execution measurement occurred.
The Linus-style skill required short files and explicit errors.
The completion-gate skill required independent outcomes and measured limits.

The separate native result API validates completion reports and repeats counterexamples.
Its contract is in `docs/native-result.md`.
The observer itself does not decide whether a search finished or whether its model covers the program.
Automatic checker construction remains outside this observer milestone.

The final matrix log is `/tmp/hyperray-observer.0TGRHa/matrix.log`.
This temporary log is local evidence, not a packaged proof certificate.
