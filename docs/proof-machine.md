# The Hyperray proof machine

Status: TARGET ARCHITECTURE 2026-09-04. The current implementation does not
satisfy this entire specification.

This document defines the language-neutral reference route. A language tool
can accelerate this route, but it cannot replace its semantic coverage proof.

## 1. Proof claim

For every accepted closed finite boundary, Hyperray examines every allowed
maximal execution for one named requirement. An execution can be finite or
infinite.

A finite execution is maximal only when its last state has no enabled
transition. Every infinite execution is maximal.

`PROVED` means that no allowed execution violates the named requirement.
`DISPROVED` means that an allowed execution violates the named requirement.

A `DISPROVED` result contains these common items:

- The requirement identifier
- The selected entry root
- The applicable state and path witness
- The cause of the violation.

The witness is a tagged union. Safety, temporal, and deadlock failures use
different witness shapes, as Section 13 specifies.

A model, coverage, compiler, tool, or proof-resource problem is an engine
error. An engine error contains no logic verdict.

The proof applies to the exact boundary, build identity, executable image, and
requirement in the result. A different item requires a different proof.

## 2. Closed finite boundary

The boundary makes the proof claim finite and closed. It names all state and
environment choices that an execution can use.

The boundary contains these facts:

- The named requirements and their finite-state monitors
- The source, runtime, libraries, build flags, target, and tool versions
- The finite domains for arguments, persistent state, and external inputs
- The rules for allocation, free, address reuse, pointer provenance,
  uninitialized data, and garbage collection
- The capacities for memory, recursion, stacks, threads, tasks, files, and
  other objects
- The rules for thread and task creation, cancellation, joining, data races,
  and atomic operations
- The allowed scheduler, clock, random, file, network, signal, and callback
  events
- The declared fairness and progress rules
- The in-scope function instances and the exact entry information for each
  instance
- The root set that each named requirement selects.

The boundary also gives the behavior at each capacity. For example, an
allocation limit can cause a modeled allocation error.

A proof-engine timeout or memory shortage is different. It is an engine error
and is not program behavior.

The state set must be finite. The transition count along an execution does not
require a fixed limit because a finite model can contain cycles.

A verifier limit is not part of the semantic boundary. An unwind count,
timeout, or proof-memory budget cannot justify `PROVED`.

An omitted input, scheduler choice, external action, or capacity behavior makes
the boundary open. Hyperray returns an engine error for that boundary.

Hyperray accepts a boundary for proof only after finite-boundary validation and
semantic coverage succeed.

## 3. Reference route

The reference route is the coverage authority for every language:

```text
source + finite boundary
  -> compiler inventory + executable image + provenance
  -> finite machine and environment transition system
  -> complete semantic coverage certificate
  -> fixed-point and accepting-cycle proof
  -> PROVED or DISPROVED
```

Each arrow has a machine-readable artifact. Each artifact records its input
digests, tool versions, target, and boundary identifier.

No later stage repairs an incomplete earlier artifact. It returns an engine
error and identifies the missing fact.

## 4. Compiler inventory and provenance

The selected compiler inventories each compiled operation in the source and
runtime. The inventory includes each concrete function instance in scope.

Compiler-generated operations also stay in the inventory. Examples include
initialization, cleanup, dispatch, and runtime operations.

Before reachability, each inventory record has one base disposition:

- `mapped` identifies its compiler output, machine instructions, and
  transitions.
- `eliminated_with_proof` identifies a checked elimination proof and the
  equivalent surviving transitions.

After one query, its proof view can refine `mapped` to
`unreachable_with_proof`. This query-specific disposition keeps the machine
mapping and its reachability proof.

The coverage checker never accepts `unreachable_with_proof` instead of a
machine mapping. Reachability cannot remove an operation from semantic
coverage.

A `mapped` inventory record has this total mapping:

```text
compiler operation
  -> compiler output record
  -> executable machine instruction or instructions
  -> machine transition or transitions
```

The compiler and linker provenance records each part of this mapping. The
executable inventory also includes linker-generated and startup instructions.

Optimization does not permit a silent omission. An eliminated operation remains
in the coverage map with `eliminated_with_proof` evidence.

Every in-scope function instance has exact standalone entry information. This
rule also applies to instances outside program-start reachability.

A function normally has a nonempty valid entry set. An impossible precondition
requires a checked proof, not a fabricated or silently empty root.

An unreachable operation stays in the inventory and provenance map. Its mapped
transitions can remain absent from the reachable-state set.

A missing operation, instruction, root, mapping, or provenance edge is a
semantic coverage error. It is never evidence for `PROVED` or `DISPROVED`.

## 5. Machine and environment transitions

A reference model is `M = (S, T, L, G)`. `S` is the finite state set.

`T` is the transition relation over `S`. `L` supplies terminal, error,
requirement, and acceptance labels.

`G` is the catalog of program and function roots. A proof query
`Q = (q, Iq)` selects requirement `q` and only its applicable roots `Iq`.

A machine state contains these parts:

- The instruction position, registers, flags, and target-machine control
  state
- The bytes, permissions, object identities, and bounds of memory
- The allocation state, freed regions, reusable addresses, pointer provenance,
  and uninitialized-data state
- The thread states, call stacks, runtime state, and scheduler state
- The garbage-collector, data-race, atomic-operation, signal, and cancellation
  state
- The finite state of each modeled environment service
- The state of each requirement monitor.

A transition gives the exact result for one machine or environment step. The
target instruction semantics define machine steps.

The selected memory model defines concurrent reads, writes, data races, and
atomic operations. Scheduler transitions keep every allowed thread choice.

Transitions model recursion, stack exhaustion, thread creation, joining,
cancellation, signals, fairness, and progress. An omitted case is an engine
error.

An environment transition can be nondeterministic only across its declared
finite result set. No stub can hide an unrestricted external action.

Unknown instructions, system calls, foreign calls, runtime actions, or
environment actions cause an engine error. Hyperray does not replace them with
an assumed value.

State storage can use hashes for lookup. Exact state equality must approve each
merge because a hash match alone is not proof evidence.

## 6. One machine route for all languages

Each language route includes its runtime in the executable image. Each route
ends at the same machine and environment transition system.

| Language | Required build content |
|---|---|
| Rust | The Rust source, selected runtime, linked libraries, compiler, linker, and target |
| C | The C source, selected runtime, linked libraries, compiler, linker, and target |
| C++ | The C++ source, selected runtime, linked libraries, compiler, linker, and target |
| Go | The Go source, Go runtime, linked libraries, compiler, linker, and target |
| Python | The Python source, pinned interpreter, runtime modules, native libraries, compiler, linker, and target |

### 6.1 First machine profile

The first profile uses a static, little-endian RV64 Linux ELF. It uses the
LP64D ABI and the RVA20U64 user profile.

The RISC-V manuals define instruction behavior. Hyperray uses one pinned Sail
RISC-V model as its executable reference for those instructions.

Hyperray produces a separate BTOR2 transition implementation. Each instruction
rule requires a one-step equivalence result against the reference semantics.

The first environment profile has one physical hart and a bounded software
thread table. This profile does not claim multicore RVWMO behavior.

The finite Linux-user environment defines each accepted syscall and external
event. It includes files, signals, futexes, time, random data, memory mappings,
and thread scheduling.

The executable has immutable code and no unresolved dynamic dependency. This
profile does not accept a dynamic loader, JIT code, or writable code page.

No existing tool supplies this full route. Hyperray must measure each profile
gate before it makes a coverage claim.

The Python compiler inventory maps each code object through interpreter
dispatch to machine instructions and transitions. The pinned interpreter is
part of the proved executable image.

Dynamic loading or code generation requires a finite model of the generator
and every permitted result. Otherwise, Hyperray returns an engine error.

Each source-language query includes the named safety requirement
`NO_UNDEFINED_BEHAVIOR`. A reachable undefined operation violates that
requirement.

Hyperray must return `PROVED` for `NO_UNDEFINED_BEHAVIOR` before it returns
another source-level `PROVED`. Undefined behavior is never a silent scope
exclusion.

## 7. Semantic coverage

Semantic coverage answers whether the transition model represents the entire
declared boundary. It does not answer which states are reachable.

A complete certificate proves these set equalities and reference checks:

- Every compiler-inventoried operation has one valid base disposition.
- Every `mapped` operation has compiler output, machine instructions,
  provenance, and transition mappings.
- Every `eliminated_with_proof` operation has a checked elimination certificate
  and equivalent surviving transitions.
- Every `unreachable_with_proof` query annotation retains its `mapped`
  operation evidence.
- Every executable instruction in scope has transition semantics and
  provenance.
- Every in-scope function instance has exact entry information or a checked
  impossible-precondition proof.
- Every proof query selects only the roots that apply to its requirement.
- Every machine transition and environment transition names supported
  semantics.
- Every model transition maps to an executable instruction or a declared
  synthetic or environment operation.
- Every state component and nondeterministic choice has a declared finite
  domain.
- Every artifact reference resolves to the exact artifact digest.

Unreachable operations remain in these equalities. A later
`unreachable_with_proof` annotation cannot drop them.

An independent coverage checker recalculates the certificate. A count, line
metric, harness list, test result, or loop list is not a coverage certificate.

Only a complete semantic coverage certificate can enter the proof engine. Every
incomplete or inconsistent certificate causes an engine error.

## 8. Reachability and the fixed point

Reachability answers which model states occur on allowed executions. It uses
the roots only after semantic coverage succeeds.

For query `Q`, the reference calculation starts with `R0 = Iq`. It applies
this equation:

```text
R(n + 1) = Rn ∪ { s' | s ∈ Rn and (s, s') ∈ T }
```

Because `S` is finite, some index `k` has `R(k + 1) = Rk`. The reachable set
is this least fixed point `R = Rk`.

Induction on path length proves that `R` contains every reachable state. The
base case places every allowed entry state in `R0`.

For the induction step, the equation adds every successor of each known
state. Thus, it contains every state on a longer path.

The reverse induction follows each added state to a predecessor and an entry
state. Thus, the fixed point contains only reachable states.

This argument proves exhaustive finite-state reachability. It does not depend
on a sampled execution or a guessed unwind limit.

## 9. Safety verdicts

Each safety requirement defines violating states and transitions. Hyperray
examines both sets over the reachable fixed point.

If no reachable violation exists, the requirement gets `PROVED`. The induction
argument covers every allowed finite execution for that query.

If a violation exists, the requirement gets `DISPROVED`. Stored predecessors
give a path from a selected entry state.

The result also gives the violating state or transition and the failed
predicate as the cause. Each named requirement gets a separate verdict.

## 10. Infinite executions and accepting cycles

An omega-regular temporal requirement uses a generalized Büchi monitor for its
negation. Hyperray forms the product of the machine model and this monitor.

A requirement outside omega-regular semantics returns an engine error unless a
separate proved reduction supplies an applicable monitor.

Hyperray first calculates the reachable product states. It then finds each
reachable strongly connected component that contains a cycle.

An accepting cycle must satisfy every acceptance set and each declared
fairness condition. A one-state component requires a self-loop.

Hyperray assumes no fairness condition that the boundary does not state.

A reachable accepting cycle gives an infinite violating execution. The
`DISPROVED` path has a finite prefix and a repeatable cycle.

If no reachable accepting cycle exists, the temporal requirement gets
`PROVED`. A violating infinite path cannot exist in a finite graph without a
repeated state.

The infinitely recurring states form a reachable component. Strong
connectivity joins their required acceptance visits into a closed accepting
walk.

Thus, the accepting-cycle search covers every allowed infinite execution.

A termination monitor accepts an infinite execution that never enters a
terminal state. Thus, a reachable nonterminal accepting cycle disproves
termination.

A maximal finite execution that ends in a nonterminal deadlock also disproves
termination. A trap or abort keeps its separate state and requirement label.

Terminal states keep explicit labels. A terminal stutter transition exists only
when the query semantics define it.

## 11. Accelerators

Kani, CBMC, ESBMC, CrossHair, Nagini, Gobra, and similar tools are
accelerators. They are not semantic coverage authorities.

An accelerator can propose invariants, state partitions, proofs, or
counterexamples. Hyperray accepts a proposal only through the reference model.

A counterexample must replay from an allowed root, transition by transition.
The replay must reproduce the named requirement violation and its cause.

An invariant must pass its base and step obligations over the reference
transitions. A translated proof requires checked, property-preserving
equivalence with the reference transition system.

An accepting-cycle witness must replay its prefix and cycle in the reference
model. Each acceptance and fairness condition must hold on that replay.

A failed equivalence check or replay is an engine error. Accelerator success
alone cannot produce `PROVED` or `DISPROVED`.

A positive accelerator result also requires an independently accepted proof
certificate. Replay validates counterexamples, but it cannot validate an
unsatisfiable result.

Kani harness selection and CBMC GOTO loop inventories can supply accelerator
data. Neither item establishes compiler inventory coverage, roots, environment
semantics, or machine coverage.

## 12. Composition and summaries

The monolithic reference transition system is the default proof subject. A
component summary is an accelerator and requires a checked equivalence
certificate.

The model defines an observation alphabet `A`. The observation function
`obs : (state, transition, state) -> A*` maps each step to its visible events.

A terminal observation function records success, deadlock, trap, abort, and
undefined behavior. Each requirement monitor reads only this observation
alphabet.

`traces` is the set of observation words from all maximal executions. It keeps
both finite words and infinite words.

For a component `C` and summary `H`, the required theorem is contextual trace
equality:

```text
for every compatible context K and applicable root e:
  traces(obs, K[C], e) = traces(obs, K[H], e)
```

The equality includes finite maximal traces and infinite traces. It preserves
each requirement-monitor label and generalized Büchi acceptance set.

The equivalence also preserves these items:

- The applicable entry roots and preconditions
- The exit states, external effects, and transition labels
- The terminal, deadlock, trap, error, and requirement labels
- The acceptance sets, fairness conditions, and internal divergence.

Each caller must satisfy the summary assumptions. Each summary records the
component, boundary, executable image, requirement, semantics, and dependency
digests.

Compatibility covers the calling convention, memory ownership, shared state,
environment assumptions, roots, exits, observations, fairness, and progress.

A changed dependency invalidates the summary. Hyperray then proves the
monolithic model or returns an engine error.

This summary theorem is a required design condition. The current implementation
does not yet establish it.

## 13. Result and error separation

The proof result contains only the logic verdicts `PROVED` and `DISPROVED`.
The operation that requests a proof can instead return an engine error.

A `PROVED` result names the requirement, boundary, executable image, compiler
inventory, semantics, coverage, proof, and trust-assumption identities. These
identities define the exact claim.

A `DISPROVED` result names the same identities. It uses one of these witness
variants:

- `safety` gives the path and the violating state or transition.
- `temporal_cycle` gives the prefix, cycle states, cycle transitions, and
  acceptance cause.
- `termination_deadlock` gives the path, nonterminal deadlock state, and cause.

Each variant contains a state, a path form, and the exact cause. The variant
also identifies the selected root.

An engine error names its stage, cause, missing references, and applicable tool
versions. It never contains a logic verdict.

Program behavior at a declared capacity is part of the transition system.
Proof-engine resource exhaustion is an engine error.

## 14. Trust boundary

The proof claim depends on a stated trust boundary. Hyperray records this trust
boundary with the result.

The trusted items are:

- The requirement monitor as a correct statement of the named requirement
- The finite boundary as a correct statement of the intended scope
- The compiler, linker, and provenance output for the source-to-executable claim
- The executable decoder and the target instruction semantics
- The machine, memory, runtime, scheduler, and environment semantics
- The coverage checker and the proof-certificate checker.

The binary-level claim uses the recorded executable image and target instruction
semantics. It does not require source fidelity.

Without checked trace preservation, the source-level claim is conditional on
trusted compiler and linker translation. A compiler-equivalence proof can
reduce this trust.

The runtime implementation is not assumed correct. Its instructions are part
of the explored machine model.

Language-specific provers are outside this trusted set after reference replay
or equivalence checks. Tests and mutation results are also outside this set.

`PROVED` is a theorem about the recorded model. A deployment claim also assumes
that the deployed executable image and environment match their recorded
identities.

## 15. Implementation status

This document is a specification. It does not claim complete semantic coverage for
the current Rust, C, C++, Go, or Python implementation.

The implementation earns this claim only after all applicable architecture,
machine, language, coverage, proof, and integration gates pass.
