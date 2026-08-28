# Ray Final Architecture

Frozen filename: `finalarchitecture.md`

Status: **FROZEN — 2026-08-27**

This file is the final architecture for Ray. It supersedes the dated design
drafts. It must not be edited after its initial freeze. A future design change
must be written as a separate successor document; this file remains immutable
so its hash continues to identify the architecture implemented and audited.

## 1. Mission

Ray verifies one fixed, bounded coding task or one fixed, bounded PR for
Python, Rust, or C++ before that task is given to a coding agent.

Ray must establish all of the following:

1. The reference solution implements the required logic.
2. No behavior outside the required logic can pass the task verifier.
3. Every behavior permitted by the required logic can pass the task verifier.
4. Candidate code cannot obtain hidden test information or modify the grading
   mechanism.
5. Every proof refers to the exact frozen task, tools, and environment that
   will be used for grading.

Ray is not a test-coverage reporter. Testing, mutation, PICT, fuzzing, and
counterexample execution are useful supporting techniques, but none may
produce Ray's mathematical all-clear.

## 2. Roles

### Task author

The task author may be a human or an AI using Ray's spec skill. The author sees
the whole task and writes `spec.md`.

Spec authoring has two ordered phases:

1. Derive the parameters, finite domains, constraints, and required behaviors
   from the instruction, base code, issue/PR, solution diff, and frozen
   environment. Do not read the tests while deciding these behaviors.
2. Freeze that semantic content, then read the tests and record which existing
   tests enforce it. Tests may reveal disagreement, but they may not create or
   change a requirement silently.

### Coding agent

The coding agent receives the normal task instruction and repository state. It
does not receive `spec.md`, hidden tests, Ray's proof model, or expected
answers.

### Ray

Ray consumes frozen task artifacts, compiles their meanings independently,
performs the proofs in this document, confirms counterexamples in the real
environment, and emits a tamper-evident certificate.

## 3. The role of spec.md

`spec.md` is Ray's strict machine-readable behavior-specification source
language.

It is not:

- a PRD shown to the coding agent;
- prose that the proof engine interprets heuristically;
- a test file;
- a list of examples;
- a second hand-written solver model; or
- `ray.toml` configuration.

Its condition tables provide the formal verification input:

1. `Parameters:` declares each bounded variable and its exact finite values.
2. Table cells select values declared by those parameter domains.
3. Each reachable full N-way parameter combination has required observable
   behavior.
4. An impossible combination is excluded only by an explicit constraint with
   frozen evidence establishing that it is unreachable.
5. Required behavior is machine-readable: returns, exceptions, effects,
   state transitions, required calls, invariants, and structural/security
   properties when those properties are part of the task.

The spec parser and spec compiler translate `spec.md` into canonical typed Spec
IR. No downstream proof component reads Markdown. There is no second
author-written "formal property" artifact: formal solver predicates are derived
from the compiled condition tables.

The source bytes, parser/compiler identity, canonical Spec IR, and their
digests are frozen together. Missing, ambiguous, partial, detached, or stale
translation evidence blocks verification.

The mathematical guarantee is relative to the behavior encoded by the frozen
Spec IR. The Phase-A author/reviewer record establishes that this
machine-readable behavior was derived from the intended task. The solver does
not infer human intent from prose.

## 4. Combination semantics and PICT

For formal verification, Ray uses the complete N-way Cartesian product of each
operation's finite parameter domains, minus only constraints whose
unreachability is established.

Grouped table cells such as `a / b` and wildcards such as `any` are compact
syntax only. The spec compiler expands them into the exact full set they
denote. Spec lint proves:

1. completeness — every reachable full N-way combination is defined;
2. disjointness — conflicting rows do not define the same combination;
3. declared values — rows introduce no undeclared domain value; and
4. constraint consistency — exclusions do not overlap reachable requirements.

PICT consumes the same parameter domains for the coverage layer. At strength
`t`, PICT generates a t-way covering array, which may omit full N-way
combinations when `t` is smaller than the number of parameters. PICT may find
test gaps and generate useful tests. Its result never proves absence of false
positives and never contributes to `VERIFIED`.

The formal layer either enumerates every full N-way combination or represents
that exact set symbolically in SAT/SMT. It never substitutes pairwise coverage,
sampling, witnesses, or mutation score for the complete set.

## 5. Mandatory task proof interface

Ray controls the accepted task format. A task that does not provide the
required proof interface is rejected before proof; Ray does not guess a
task-specific mapping.

Every task provides a frozen proof harness for each operation. The harness
defines:

1. the real implementation entry point;
2. the typed mapping from every spec parameter value to the actual input or
   initial program state it represents;
3. reset and call-sequence rules;
4. the observations that constitute returns, exceptions, state changes,
   effects, and required calls; and
5. the boundary through which the task verifier interacts with candidate code.

The harness is generic infrastructure data, not a per-task mutation recipe and
not a copy of the expected answer. It connects the machine-readable spec
vocabulary to real program values and observations.

The implementation behind the harness may use ordinary functions, methods,
classes, loops, recursion, containers, and dependencies. Those constructs are
handled by the selected language verifier under the fixed task bounds. Ray does
not approximate an unsupported construct. The author must constrain or annotate
the task into the supported proof profile, or Ray rejects the task.

`ray.toml` contains wiring only: artifact identifiers, paths, exact commands,
tool identities, sandbox capabilities, timeouts, pass-signal location, and
certificate location. Parameter domains and required behavior never originate
from `ray.toml`.

## 6. Frozen artifacts

Before translation, Ray freezes and hashes:

- `instruction.md`;
- `spec.md` and the Phase-A author/reviewer record;
- repository identity and exact base commit;
- issue/PR metadata supplied by the task;
- solution and test patches;
- base, test, and solution workspace derivations;
- proof harnesses and verifier commands;
- compiler, interpreter, solver, and supporting tool binaries and versions;
- declared dependencies;
- environment configuration and sandbox policy; and
- the authoritative pass signal.

Prepared workspaces must be reproducibly derived from the frozen base and
patches. Ray does not trust an arbitrary prebuilt directory merely because its
path appears in configuration.

## 7. Independent semantic models

Ray builds four independent models before proving their relationships:

1. **Spec IR** — domains, constraints, required observations, invariants, and
   structural/security requirements compiled only from the frozen spec source.
2. **Code IR** — the reference implementation and proof-harness behavior,
   translated from real Python, Rust, or C++ artifacts without copying expected
   outcomes from Spec IR.
3. **Test IR** — the task verifier's complete acceptance predicate, translated
   from real test/verifier artifacts without copying expected outcomes from
   Spec IR.
4. **Environment and Security IR** — permitted information sources, effects,
   capabilities, reset rules, toolchain behavior, and pass-signal ownership.

These models share typed identifiers only after independent translation.
Disagreement is a finding. No majority vote repairs a disagreement.

The language frontends must preserve every task-relevant construct. They may
use compiler-produced representations and established solvers/model checkers.
Translation coverage is proof evidence, not a percentage: every reachable
construct is either translated faithfully or produces `PROOF BLOCKED`.

## 8. Complete behavior model

For each reachable full N-way parameter combination, Ray creates one behavior
point. A behavior point includes the exact input/state assignment and its
complete observable result.

The outcome alphabet includes every task-relevant declared result and a closed
rejection path for undeclared returns, undeclared exceptions, missing output,
crash, timeout, and forbidden effects. Such failures cannot fall outside the
model silently.

A complete candidate behavior is a total mapping from every behavior point to
one outcome. Stateful tasks additionally include the declared finite state and
transition sequence. Test IR is a predicate over the complete global behavior,
not a union of independent test names or per-row guesses. It therefore
preserves ordering, shared state, cross-case comparisons, and relational
assertions.

SAT/SMT may represent this behavior space symbolically. Explicit enumeration
is not required when the symbolic representation is exact.

## 9. Formal proof obligations

Let:

- `B` be the complete bounded behavior model;
- `R(b)` mean that behavior `b` satisfies the required behavior compiled from
  the condition tables;
- `C(b)` mean that the reference implementation can produce `b`; and
- `T(b)` mean that the frozen task verifier accepts `b`.

### 9.1 Reference logic correctness

Ray proves that no reference behavior falls outside the required behavior:

```text
B(b) AND C(b) AND NOT R(b)
```

The formula must be `UNSAT`.

### 9.2 False-positive freedom

Ray proves that no incorrect behavior passes the task verifier:

```text
B(b) AND T(b) AND NOT R(b)
```

The formula must be `UNSAT`. A satisfying assignment is a concrete false
positive: behavior that the tests accept even though the required-behavior
tables reject it.

This is exactly test-oracle soundness: `T(b) implies R(b)`.

### 9.3 Verifier fairness and completeness

Ray proves that the verifier does not reject behavior permitted by the task:

```text
B(b) AND R(b) AND NOT T(b)
```

The formula must be `UNSAT`.

This is test-oracle completeness: `R(b) implies T(b)`.

### 9.4 Noninterference

Ray proves that candidate behavior cannot depend on forbidden grading context.
For two executions with identical declared task inputs and state but different
hidden test context:

```text
declared_input_1 = declared_input_2
AND forbidden_context_1 != forbidden_context_2
AND behavior_1 != behavior_2
```

The formula must be `UNSAT`.

Forbidden context includes hidden tests, expected answers, test identity,
verifier internals, and undeclared environment state. Static information-flow
evidence and the runtime sandbox jointly establish this property.

### 9.5 Grader integrity

Ray proves and enforces that candidate code cannot modify or forge:

- test/verifier artifacts;
- expected-answer artifacts;
- Ray's supervisor;
- result files or the authoritative pass signal;
- proof transcripts; or
- certificates.

The candidate receives only declared inputs and allowed capabilities. Hidden
grader artifacts are outside its readable and writable namespace.

### 9.6 Required implementation structure

When the task requires more than input/output equivalence, the spec declares
the structural property in machine-readable form: required or forbidden calls,
allowed dependencies, effects, state transitions, complexity bounds, or
information-flow restrictions. Code IR must prove those properties.

This is how a task can reject embedded answer tables or other prohibited
shortcuts. Ray does not guess whether behavior is "cheating" from style; the
task format states the allowed structure and Ray proves it.

## 10. How the verifier predicate is obtained

Ray does not infer test enforcement from test names, shared words, or line
coverage.

At the frozen proof-harness boundary, Ray derives the task verifier's acceptance
predicate using compiler-backed Test IR or an exact symbolic/table
substitution. The derived predicate includes the real pass-signal calculation
and all tests that contribute to it.

For auditing an existing task, Ray proves Sections 9.2 and 9.3 against the
existing verifier and reports counterexamples.

For a Ray-corrected task, Ray may generate the authoritative hidden verifier
from canonical Spec IR. Ray then proves that the generated verifier predicate
is identical to `R`. Existing project tests may remain as additional checks,
but they may affect the grading result only after the fairness proof establishes
that they reject no permitted behavior.

## 11. Preventing cheating by construction

The proof environment applies all of these controls:

1. Candidate code and the grader execute in separate security domains.
2. Candidate code receives only declared inputs through the proof interface.
3. Hidden tests, `spec.md`, expected outputs, verifier identity, and proof data
   are not readable by candidate code.
4. Test and result artifacts are not writable by candidate code.
5. Network, ambient environment, clock, randomness, reflection, dynamic
   loading, and undeclared files are denied unless the task explicitly models
   them.
6. Each independent call begins from the declared reset state; cross-call state
   exists only when the task specification includes it.
7. Static data-flow/information-flow analysis checks for forbidden sources and
   sinks.
8. The runtime supervisor records canonical requests, observations, exits,
   crashes, and timeouts.
9. Repeated execution and frozen scheduling rules detect undeclared
   nondeterminism when execution evidence is used.
10. Artifact hashes and sandbox evidence are certificate-bound.

These controls define Ray's threat model. A property outside this model cannot
be claimed by the certificate.

## 12. Solvers and language frontends

Ray supports Python, Rust, and C++ through language-specific, compiler-backed
frontends that lower the accepted proof profile into the shared Semantic IR and
SAT/SMT obligations.

The frontend contract is the same for every language:

1. use the frozen native compiler/interpreter and dependency set;
2. translate every reachable construct in the proof harness and changed scope;
3. preserve language-specific values, exceptions, undefined behavior, effects,
   calls, and control flow;
4. establish sufficient loop, recursion, allocation, and state bounds;
5. emit replayable derivation and coverage evidence; and
6. reject, rather than approximate, unsupported behavior.

Compiler IR is an implementation mechanism, not the source of task
requirements. It supplies program semantics. Spec IR supplies required
behavior. Test IR supplies verifier acceptance. The proof checks their
relationships.

## 13. Counterexample confirmation

A solver counterexample is translated back into exact program inputs, state,
outcomes, and verifier observations. Ray executes it in a fresh copy of the
frozen real environment.

If real execution disagrees with the model, Ray does not discard the witness or
claim success. It reports a Ray translation/model mismatch and returns
`PROOF BLOCKED` until that mismatch is resolved.

Counterexample execution validates the implementation of the formal model. It
does not replace the universal proof.

## 14. Verdicts

Ray has exactly three proof-level verdicts:

- `VERIFIED` — every mandatory proof is complete and all formulas requiring
  emptiness are `UNSAT`.
- `NOT VERIFIED` — at least one proof has a confirmed counterexample.
- `PROOF BLOCKED` — required evidence, translation, isolation, solver
  completeness, or execution confirmation is missing or inconsistent.

`UNKNOWN`, timeout, skipped analysis, unsupported syntax, stale evidence,
sampling success, killed mutants, or partial coverage can never render as
`VERIFIED`.

## 15. Certificate and trusted computing base

The certificate contains or binds:

- all frozen artifact and workspace hashes;
- Phase-A spec-authoring evidence;
- Spec IR, Code IR, Test IR, and Environment/Security IR digests;
- proof-harness and point-universe digests;
- compiler, interpreter, solver, supervisor, and generator identities;
- all proof formulas, results, and completeness evidence;
- counterexamples and real-execution confirmations;
- noninterference and integrity evidence; and
- the final verdict.

Certificate verification recomputes canonical digests and rejects detached,
stale, missing, reordered, truncated, or tampered evidence.

Ray's trusted computing base consists of the frozen spec compiler, language
frontends, proof-harness supervisor, sandbox enforcement, SAT/SMT solver,
certificate generator, and certificate verifier. Ray never claims stronger
assurance than this trusted computing base supports.

## 16. End-to-end pipeline

```text
Fixed task or PR
        ↓
Author spec.md from instruction + base + issue/PR + solution + environment
        ↓
Freeze Phase-A semantic content
        ↓
Read tests and attach enforcement mappings without changing semantics
        ↓
Freeze repository, patches, workspaces, tools, environment, and pass signal
        ↓
Compile spec.md → canonical Spec IR
        ↓
Independently compile reference implementation → Code IR
        ↓
Independently compile verifier/tests → Test IR
        ↓
Compile sandbox and capability policy → Environment/Security IR
        ↓
Build the complete bounded behavior model
        ↓
Prove reference correctness
        ↓
Prove false-positive freedom
        ↓
Prove verifier fairness/completeness
        ↓
Prove noninterference and grader integrity
        ↓
Execute every counterexample in the frozen real environment
        ↓
Issue a tamper-evident certificate
        ↓
Only VERIFIED tasks may enter the evaluation system
```

## 17. Techniques that never prove the all-clear

The following may find bugs or guide test creation but cannot contribute to a
`VERIFIED` proof result:

- PICT pairwise or t-way coverage below full N-way;
- source-line, branch, or token coverage;
- mutation score or a finite mutant family;
- fuzzing or property-based sampling;
- regex, keyword, embedding, or test-name matching;
- an LLM's unsupported semantic judgment;
- one successful reference run; or
- any finite sample substituted for an exact symbolic proof over a larger set.

These mechanisms may supply counterexamples. They may not establish that no
counterexample exists.

## 18. Completion standard for Ray

Ray is not complete until the production CLI performs this exact architecture
end to end and passes all of the following:

1. A real Python task with a correct verifier returns `VERIFIED`.
2. A real Rust task with a correct verifier returns `VERIFIED`.
3. A real C++ task with a correct verifier returns `VERIFIED`.
4. Real tasks with missing tests produce confirmed false-positive witnesses.
5. Over-restrictive tests produce confirmed fairness witnesses.
6. Incorrect reference solutions produce confirmed correctness witnesses.
7. Test-reading, result-forging, hidden-state, and nondeterminism adversaries
   cannot obtain `VERIFIED`.
8. Spec, source, test, environment, tool, transcript, and certificate tampering
   is detected.
9. Unsupported or incomplete translation returns `PROOF BLOCKED`.
10. The complete repository test suite, static analysis, certificate replay,
    and frozen real-task gates pass from a clean invocation.

No component test, smoke test, synthetic fixture, sampled agreement, or
partially wired pipeline is sufficient for completion.
