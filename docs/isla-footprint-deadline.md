# Footprint host deadline

The caller supplies a time limit for each instruction operation.
Hyperray passes this limit to Isla and creates a host context with the same limit.
An earlier caller deadline takes precedence.

The pinned upstream tool supplies a cooperative timeout.
`src/footprint.rs` passes the timeout to `executor::start_multi`.
`isla-lib/src/executor.rs` examines this timer between interpreter instructions.
A long operation inside an instruction does not return to that timer immediately.

Before this change, `runFootprint` used the caller context without the request time limit.
A live footprint process continued for more than nine minutes with `--timeout 120`.
This observation does not identify the specific internal operation that delayed its timer.

The host deadline cancels the direct child process.
`command.Run` waits for that process before Hyperray returns `resource_limit`.
The result contains no partial footprint report, even when earlier instructions succeeded.
The request does not omit the slow instruction or reduce the coverage requirement.

`footprint_deadline_test.go` uses the existing fake footprint tool for ordinary output.
A wrapper supplies a nonterminating operation that ignores the tool timeout.
The tests measure the host deadline, an earlier caller deadline, process termination, and duration overflow.
