# Plan: explicit executable threads

Status: implementation authorized for this bounded connection only.

## Contract and evidence

`docs/design.md` and `docs/proof-machine.md` define the accepted target architecture. `PLAN.md` retains the full finite-boundary objective. Commit `c272c53` removed the old frozen documents. Do not restore those documents or use them as current authority.

The current builder writes only `[thread.0]`. `ProgramBoundary` describes one entry. `matchProgramSemantics` accepts only one reported thread. The event reader already parses a contiguous sequence of numbered threads. Isla worker parallelism is not modeled program concurrency.

This task connects an explicit finite set of initial machine threads to the existing axiomatic backend. It does not implement Rust thread creation, infer a scheduler, establish fairness, or prove full Rust coverage. Do not claim those results.

## Required public behavior

1. Keep the existing single-thread public API and its behavior compatible.
2. Add a public thread-entry type and a public construction path for a nonempty ordered set of threads in one loaded ELF. Each thread declares its entry address and concrete initial registers. Order defines contiguous thread IDs.
3. Validate each entry against the loader's actual instruction-start inventory, not merely an address range. Repeated entry addresses are legal because distinct threads can run the same function.
4. Validate register assignments independently per thread. Do not mutate caller slices. Reject ambiguous mixtures of legacy entry/register fields and explicit thread fields.
5. Generate each loaded section once. Use the native thread input grammar. Do not synthesize instruction behavior or clone shared memory into independent private copies.
6. Reject sequential memory with more than one modeled thread before tool execution. Never silently substitute a memory model. Retain single-thread sequential support.
7. Retain the exact declared thread entries in opaque program identity and public evidence. Require reported thread count to match the declared count. Missing or extra threads return an error without a verdict. Retain per-thread identity in semantic reports and execution inventories. Each declared thread must have its own entry instruction event. An aggregate instruction set must not let one thread satisfy another thread's entry requirement. Reject missing, duplicate, reordered, or extra thread records. Related semantic report, summary, thread parser, execution inventory, and test files are within implementation ownership for this requirement.
8. Reject unsupported typed-state combinations explicitly unless native source and an execution test establish their meaning. Do not invent per-thread typed-state syntax.

## Implementation ownership

The coding agent can change `machine/isla/program*.go`, `machine/isla/executable_coverage.go`, directly related identity/join code, and their tests. It can add a focused real-tool thread test by reusing the existing public test helpers. It can update `docs/isla-explicit-threads.md` and `gates/leaf-isla-explicit-threads.md`. Do not change native engine source, unrelated adapters, architecture scope, release files, or existing user changes. Keep related helpers together where practical.

Read the workspace workflow, applicable project instructions, and the linus-code-style, simple-english, outside-caller, test-driven-development, and verification-before-completion skills before implementation. No commits or pushes.

## Acceptance

- Public external-package tests construct one and two threads, inspect evidence and generated input, and establish deterministic output without caller mutation.
- Negative tests reject zero explicit threads, non-instruction entries, conflicting legacy fields, duplicate registers within a thread, multiple sequential threads, insufficient output limits, and missing/extra reported threads. Duplicate entries across threads remain valid.
- Existing single-thread tests pass unchanged except assertions whose documented evidence gains additive fields.
- A real native parser/executor accepts generated two-thread input from compiled code through the public SDK. Use the existing axiomatic memory model and release/configuration helpers. Require two semantic thread trees and a result that observes both threads. A changed requirement must yield a concrete counterexample.
- Add a shared-memory test only when the existing axiomatic backend can faithfully represent it. Report a native limitation exactly. Do not bypass it with sequential execution, hand-authored result fixtures, or per-program semantics.
- Run `go test ./machine/isla`, `go test ./...`, `go vet ./...`, and the exact real-tool command selected from the current helper documentation. Record commands, exit codes, logs, and skipped cases separately. Inspect disk space before builds. A skipped real-tool test does not pass the real-tool requirement.
- A separate review must inspect API reachability, shared-memory meaning, thread identity, error behavior, and scope claims before completion.

## Review repairs and remaining acceptance

The native TOML parser iterates thread keys in lexical order. For 11 or more threads, use equal-width decimal keys so lexical order matches caller order. Preserve existing output for fewer than 11 threads. A real 11-thread test must use identical entry addresses and different register inputs. It must observe thread 2 and thread 10 independently.

The first instruction is branch-local until a shared prefix executes it. Retain every possible first instruction address, independent of sibling traversal order. Executable matching must reject a thread if any branch starts at a different declared entry. Test both sibling orders, equal-entry forks, and a shared instruction prefix.

Counterexample tests must assert actual register values, not merely the presence of register names. The branch fixture requires thread 0 result 17 and thread 1 result 8.

Add a compiler-built shared-memory acceptance fixture after those repairs. Two explicit threads use the same static `AtomicU64`, initialized to zero. One thread stores 1, and the other loads the value. Use Rust atomic operations, not a non-atomic data race. Reuse the existing axiomatic backend and initial loaded section. Prove that the reader returns only 0 or 1. Require separate reachable counterexamples for 0 and 1. Inspect the generated ELF and trace to establish that both threads access the same loaded address. Do not replace shared memory with private copies or manually supply a thread result. This fixture measures existing axiomatic event connections. It does not establish the target software-thread scheduler.

The coding agent can add this Rust fixture under `fixtures/rust/machine` and its integration test under `machine/isla`. Native engine changes remain outside this task. A native failure must remain visible and becomes input to the next design decision.

## Review prevention rules

Worker count, modeled thread count, and instruction-visit limit are three separate values. Never compare them or derive one from another. Confirm each native argument's meaning from its parser before adding a guard. This confusion occurred during planning and recurred in implementation. Acceptance now requires 11 modeled threads with one worker and an independent visit bound.

Each negative test starts from an accepted baseline. Change only the property under test. Require its specific error, not any error. A test with two invalid fields cannot establish which check worked.

A review repair twice changed aggregate inventory instead of the thread-entry inventory. This tested the union guard, not the named entry guard. Before each mutation, name the intended production error branch. Retain every prerequisite for that branch. For a missing thread-entry event, remove the thread's instruction list but retain its entry metadata and the aggregate list. Assert `thread entry event missing`. A different error does not satisfy this test.

A shared-address claim requires a memory event in each named thread. Repeated text anywhere in the combined trace is insufficient evidence.

## Stop conditions

Return a precise implementation blocker if native grammar or memory semantics contradict this contract. Do not weaken the contract to make tests pass. An implemented API with an unpassed real-tool test remains incomplete. The full coverage objective remains open regardless of this leaf's outcome.
