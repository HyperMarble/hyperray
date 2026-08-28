# Ray Whole Flow

Frozen filename: `whole flow.md`

Status: **FROZEN — 2026-08-27**

This file defines the complete operational flow that implements
`finalarchitecture.md`. It includes authoring, freezing, translation, proof,
security enforcement, counterexample confirmation, remediation, certificates,
CLI behavior, and completion gates. It must not be edited. A later change
requires a separate successor document.

## 1. Inputs to one Ray run

One run verifies one fixed task or PR. The input bundle contains:

```text
task/
  instruction.md
  spec.md
  ray.toml
  repository metadata and exact base commit
  issue or PR metadata
  solution patch
  test patch
  proof harnesses
  verifier/test commands
  environment definition
  declared dependencies and tools
```

The files have separate responsibilities:

| Artifact | Responsibility |
|---|---|
| `instruction.md` | Normal human/agent-facing task description |
| `spec.md` | Ray machine-readable parameter domains, constraints, and required behavior tables |
| `ray.toml` | Paths, commands, tool identities, sandbox capabilities, timeouts, pass signal, and certificate location |
| base commit | Exact repository state before the task |
| solution patch | Reference implementation change |
| test patch | Task verifier change |
| proof harness | Typed connection between spec values and real program inputs, state, calls, and observations |
| environment | Exact compiler/interpreter, dependencies, operating conditions, and isolation policy |

`ray.toml` never supplies a requirement, parameter value, or expected answer.

## 2. Phase A — author the machine-readable behavior

The task author uses the spec skill and may be a human or AI.

The author reads:

1. `instruction.md`;
2. the exact base code;
3. the issue or PR scope;
4. the solution diff;
5. every changed branch, boundary, exception, effect, state transition, and
   relevant caller; and
6. the frozen environment and dependency behavior relevant to the task.

The author does not read the tests while deciding the semantic content.

For each required operation, the author writes:

1. the bounded parameters;
2. every exact value in each parameter domain;
3. explicit constraints for impossible full N-way combinations;
4. required returns, exceptions, effects, state changes, calls, and invariants;
5. structural or information-flow requirements when implementation method
   matters; and
6. the instruction/code/PR provenance that justifies each row.

The condition tables must cover the full N-way Cartesian product after proven
constraints are removed. Grouped cells and `any` are only compact syntax.

The author runs strict spec lint after each table. Strict lint rejects:

- an uncovered full N-way combination;
- two conflicting rows for the same combination;
- a value absent from its declared domain;
- an empty domain;
- an unsupported or ambiguous behavior expression;
- an exclusion without a reason and reachability evidence;
- a requirement outside a machine-readable table; or
- a requirement with missing provenance.

When Phase A is complete, Ray freezes:

- `spec.pretest.md`;
- its semantic digest;
- its exact source digest;
- the instruction/base/solution/environment digests used during authoring; and
- the Phase-A author/reviewer record.

## 3. Phase B — map the existing tests

Only after Phase A is frozen does the author read the tests.

For each already-frozen behavior row, the author records the test or verifier
observation intended to enforce it. Missing enforcement is recorded as
missing; it is never filled by inventing a textual match.

The final `spec.md` may differ from `spec.pretest.md` only in test-enforcement
evidence. If a semantic row, parameter, constraint, or required behavior must
change, Phase A restarts and receives new hashes and review.

Ray freezes the final spec source and proves that its semantic digest equals
the Phase-A semantic digest.

## 4. Start and check enter the same pipeline

`ray start <task>` and `ray check <task>` call the same production pipeline.

`ray start` may initialize missing wiring artifacts, but it does not use a
weaker verifier and cannot return success before the complete pipeline runs.

Both commands:

1. load strict `ray.toml`;
2. reject unknown fields and undeclared ambient configuration;
3. resolve every artifact by ID and expected kind;
4. invoke the same freeze, translation, proof, execution, and certificate
   stages; and
5. exit successfully only for `VERIFIED`.

## 5. Freeze and reproduce the real task

Ray resolves the exact repository commit using the frozen VCS tool. It creates
fresh workspaces and derives them mechanically:

```text
base workspace               = exact base commit
base + new tests workspace   = base commit + test patch
solution + new tests         = base commit + solution patch + test patch
```

If legacy tests are part of the real grader, their exact state is frozen as a
separate artifact. Ray never fabricates a replacement test to make a workspace
pass.

For every workspace Ray records:

- repository and tree digests;
- ordered patches and patch digests;
- file inventory and content digests;
- exact command argument arrays;
- exact working directory;
- declared environment variables;
- tool binary hashes and versions;
- timeout and resource limits; and
- authoritative pass-signal behavior.

Commands do not inherit undeclared ambient environment. A prepared workspace
that cannot be reproduced from the frozen base and patches blocks proof.

## 6. Parse, lint, and compile spec.md

The spec parser reads only the strict source grammar. It returns structured
tables, not proof conclusions.

The spec compiler:

1. validates every parameter domain;
2. expands grouped cells and wildcards;
3. constructs the complete full N-way combination set;
4. validates constraints and removes only proven-unreachable combinations;
5. parses required outcomes, effects, state transitions, calls, invariants,
   and structural/security requirements;
6. links provenance without treating provenance as proof of meaning;
7. emits canonical typed Spec IR; and
8. emits complete source-to-IR derivation evidence.

No other component reads Markdown. A second compilation with the same frozen
inputs must produce byte-identical canonical Spec IR.

## 7. Validate the proof harness

For each operation, Ray validates the frozen proof harness independently of
the required outcomes.

The harness must provide:

1. one exact real entry point;
2. a typed representation for every declared spec value;
3. the mapping from parameter combinations to actual inputs and initial state;
4. reset and call-sequence behavior;
5. all observable return, exception, state, effect, and call channels;
6. exact behavior for crash, timeout, and missing output; and
7. a closed boundary between candidate code and the verifier.

The harness cannot contain the expected behavior table. Its job is only to map
values, invoke real code, and report observations.

Ray proves harness closure:

- the entry point resolves to the intended changed operation;
- every declared input is reachable through the harness;
- no undeclared candidate input channel exists;
- no task-relevant output or effect bypasses observation; and
- the verifier reaches candidate code only through the declared boundary.

## 8. Build four independent IRs

### 8.1 Spec IR

Produced only from frozen `spec.md`. It contains domains, full N-way behavior
points, constraints, required observations, invariants, and structural/security
requirements.

### 8.2 Code IR

Produced from the real reference implementation, solution patch, proof
harness, compiler/interpreter, and dependencies. The code frontend receives
typed parameter identities and harness entry points, but it does not receive
required outcomes as a shortcut for translating implementation behavior.

### 8.3 Test IR

Produced from the real tests, verifier, proof harness, runner, and pass signal.
It represents the complete global acceptance predicate. It does not receive
required outcomes as a shortcut for translating assertions.

### 8.4 Environment and Security IR

Produced from frozen tools, environment, sandbox, reset policy, filesystem
policy, network policy, clock/randomness policy, process boundary, and pass
signal ownership.

The four IRs remain separate until relationship proofs begin. Their artifact,
tool, and derivation digests must match the frozen run.

## 9. Language frontend flow

The same frontend contract applies to Python, Rust, and C++:

1. invoke the frozen native compiler or interpreter;
2. obtain compiler-backed source/bytecode/MIR/LLVM or model-checker semantics;
3. lower every reachable task-relevant construct to shared Semantic IR;
4. preserve values, exceptions, calls, effects, state, undefined behavior, and
   control flow;
5. establish sufficient finite bounds for loops, recursion, allocation, and
   state;
6. emit source-to-IR coverage and replay evidence; and
7. reject any unsupported reachable construct.

The task author may simplify or annotate code to meet this fixed proof profile.
Ray does not add per-task semantics, hardcoded function names, hand-written
evaluators, or guessed runtime values.

## 10. Assemble the complete bounded behavior model

Ray joins the independently produced IRs only through validated typed IDs.

For every reachable full N-way parameter combination, Ray creates one exact
behavior point. Each point has:

- its operation;
- exact typed input and initial state;
- its complete local outcome alphabet;
- required and forbidden observations;
- effects and invariants; and
- source and derivation evidence.

The local outcome alphabet is closed. Any undeclared return, exception, effect,
crash, timeout, or missing output maps to a rejected outcome instead of
disappearing.

A complete candidate behavior assigns one outcome to every behavior point.
Stateful tasks additionally assign the declared state transition sequence.

The complete behavior space is represented either:

- by exact enumeration; or
- by a logically equivalent symbolic SAT/SMT representation.

Resource limits may cause `PROOF BLOCKED`; they never justify sampling.

## 11. Derive the verifier acceptance predicate

Test IR represents whether the real verifier passes for a complete candidate
behavior.

Ray derives it using one or both exact mechanisms:

1. compiler-backed translation of the verifier and its assertions; or
2. symbolic/table substitution at the closed proof-harness boundary, executing
   the frozen verifier against arbitrary complete behavior tables.

The predicate includes:

- every grading test;
- assertion conjunction and disjunction;
- setup, teardown, ordering, and shared state;
- cross-input and relational checks;
- process exit and result files; and
- the exact authoritative pass-signal calculation.

Test names, source tokens, embeddings, line coverage, and row-to-test guesses
never define the predicate.

## 12. Run the formal proofs

Let:

```text
B(b) = b belongs to the complete bounded behavior model
R(b) = b satisfies the required behavior compiled from spec.md
C(b) = the reference implementation can produce b
T(b) = the frozen verifier accepts b
```

Ray runs every mandatory proof.

### 12.1 Reference correctness

```text
B(b) AND C(b) AND NOT R(b)
```

Required result: `UNSAT`.

`SAT` produces a reference-logic counterexample.

### 12.2 False-positive freedom

```text
B(b) AND T(b) AND NOT R(b)
```

Required result: `UNSAT`.

`SAT` produces an incorrect behavior that the verifier accepts.

### 12.3 Verifier fairness and completeness

```text
B(b) AND R(b) AND NOT T(b)
```

Required result: `UNSAT`.

`SAT` produces permitted behavior that the verifier rejects.

### 12.4 Candidate noninterference

Ray creates two symbolic executions with identical declared inputs/state and
different forbidden grading context:

```text
declared_input_1 = declared_input_2
AND forbidden_context_1 != forbidden_context_2
AND candidate_behavior_1 != candidate_behavior_2
```

Required result: `UNSAT`.

### 12.5 Grader integrity

Ray proves through static capability/data-flow checks and sandbox evidence that
candidate code has no read or write path to hidden tests, expected answers,
verifier internals, result files, pass signals, proof data, or certificates.

### 12.6 Structural requirements

When Spec IR declares required/forbidden calls, allowed dependencies, state
transitions, effects, complexity limits, or anti-hardcoding rules, Code IR must
satisfy them for every bounded execution.

### 12.7 Translation and solver completeness

Every proof records:

- formula digest;
- solver and frontend identity;
- exact bounds;
- complete translated-construct inventory;
- result and model/proof evidence; and
- whether the result is exhaustive.

`UNKNOWN`, unsupported behavior, incomplete unwinding, missing evidence,
resource exhaustion, or stale translation returns `PROOF BLOCKED`.

## 13. Existing-verifier audit mode

Ray first audits the task's existing verifier.

If the false-positive formula is `SAT`, Ray reports the exact missing behavior
and produces the accepted incorrect behavior table.

If the fairness formula is `SAT`, Ray reports the exact permitted behavior the
verifier rejects.

Ray does not automatically reinterpret `spec.md` to make the verifier pass.
The author chooses whether the defect is in the tests, reference solution, task
instruction, or authored spec. Any semantic spec change restarts Phase A.

## 14. Corrected-verifier generation mode

After Spec IR is frozen, Ray may generate an authoritative hidden verifier
directly from Spec IR and the proof harness.

The generator:

1. queries every behavior point or uses an exact symbolic equivalent;
2. observes every declared output/effect channel;
3. rejects undeclared, crash, timeout, and missing-output behavior;
4. applies the required-behavior predicate exactly; and
5. owns the authoritative pass signal outside candidate control.

Ray rebuilds the generated verifier from frozen inputs and proves byte and
predicate identity. Its acceptance predicate must equal `R`.

Existing project tests may run as extra diagnostics. They may contribute to the
grading result only after Ray proves they reject no `R`-permitted behavior.

## 15. Enforce the anti-cheating boundary

The runtime supervisor launches grader and candidate in separate security
domains.

Candidate access is limited to:

- declared task inputs;
- declared initial state;
- explicitly allowed files and dependencies; and
- explicitly allowed effects.

Candidate access excludes:

- hidden tests and test identity;
- `spec.md` and expected answers;
- verifier source and control data;
- result files and pass signals;
- proof transcripts and certificates;
- undeclared environment variables and files;
- network, clock, randomness, reflection, and dynamic loading unless modeled.

The supervisor uses fresh state according to the task's reset policy, records
canonical requests and observations, and detects undeclared nondeterminism.

Static information-flow analysis and runtime capability enforcement must agree.
Either one reporting an open forbidden channel blocks proof.

## 16. Confirm every counterexample in the real environment

For every solver witness, Ray materializes the exact real input/state and runs:

1. the reference implementation when reference correctness failed;
2. the existing verifier against the witness behavior when soundness or
   fairness failed; and
3. both security contexts when noninterference failed.

Execution occurs in a fresh derived workspace with the frozen commands,
environment, tools, timeout, and pass signal. Ray restores and rehashes the
workspace afterward.

Possible results:

- model and execution agree — the finding is confirmed;
- execution cannot be reproduced — `PROOF BLOCKED`;
- execution disagrees with the model — Ray frontend/model defect,
  `PROOF BLOCKED`.

A failed confirmation never becomes `VERIFIED`.

## 17. Remediation loop

Ray findings are repaired according to their type:

| Finding | Required action |
|---|---|
| reference correctness counterexample | fix the solution or correct Phase-A semantics and restart authoring |
| false-positive counterexample | strengthen/fix the verifier, or correct Phase-A semantics and restart authoring |
| fairness counterexample | remove/fix the over-restrictive verifier behavior, or correct Phase-A semantics and restart authoring |
| noninterference counterexample | close the forbidden information channel and rerun freeze |
| integrity failure | fix sandbox/permissions/pass-signal ownership and rerun freeze |
| unsupported translation | constrain/annotate the task into the fixed proof profile or extend Ray generically |
| stale/tampered evidence | discard and regenerate evidence from frozen inputs |
| model/execution disagreement | fix Ray's frontend, harness validation, or supervisor; never patch the task to hide it |

The complete pipeline reruns after every repair. Previous proof results cannot be
reused across changed semantic or artifact digests.

## 18. Supporting diagnostic layers

These layers run outside the mathematical all-clear:

### PICT coverage

Generates configured t-way covering arrays from the spec domains and identifies
useful missing test scenarios. A surviving gap is a diagnostic. A clean PICT
result is not proof.

### Mutation

Applies a finite family of source changes and runs the verifier. A surviving
mutant is a useful concrete finding. Killing every generated mutant is not
proof.

### Fuzzing and property-based testing

May discover concrete failures. Sample agreement is not proof.

### Coverage, token matching, and AI review

May guide human attention. They do not define requirement enforcement and
cannot contribute to `VERIFIED`.

## 19. Issue the certificate

Ray issues a certificate only after all mandatory proofs and confirmations
complete.

The certificate binds:

1. task and architecture identifiers;
2. source, patch, repository, workspace, and environment hashes;
3. Phase-A and final spec source plus semantic equality evidence;
4. Spec IR, Code IR, Test IR, and Environment/Security IR;
5. proof harnesses and boundary-closure evidence;
6. compiler/interpreter/frontend/solver/supervisor identities;
7. exact formulas, bounds, results, and proof evidence;
8. noninterference and integrity evidence;
9. counterexamples and real-execution confirmations;
10. generated-verifier identity when used; and
11. final verdict.

Certificate verification independently recomputes canonical digests and
cross-bindings. Missing, detached, reordered, truncated, stale, or tampered
evidence is rejected.

## 20. CLI verdict and exit behavior

Ray returns exactly one proof verdict:

```text
VERIFIED
NOT VERIFIED
PROOF BLOCKED
```

`VERIFIED` requires:

- complete and consistent Spec IR;
- complete Code IR, Test IR, and Environment/Security IR;
- reference correctness `UNSAT`;
- false-positive formula `UNSAT`;
- fairness formula `UNSAT`;
- noninterference `UNSAT`;
- grader integrity established;
- all required structural properties established;
- all counterexamples, if any, resolved by a new full run;
- certificate issuance and verification success; and
- no stale or unsupported evidence.

`ray start` and `ray check` exit zero only for `VERIFIED`. Every other verdict
exits nonzero and identifies the exact failed or blocked stage.

## 21. Complete data flow

```text
instruction + base + issue/PR + solution + environment
                         │
                         ▼
             Phase-A spec skill authoring
                         │
                         ▼
                frozen spec.pretest.md
                         │
              tests consulted only now
                         │
                         ▼
        final spec.md + enforcement mapping
                         │
                         ▼
 repository / patches / tools / environment freeze
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
     speccompiler   code frontend   test frontend
          │              │              │
          ▼              ▼              ▼
       Spec IR         Code IR         Test IR
          │              │              │
          └──────────────┼──────────────┘
                         │
 environment + sandbox ─┤
                         ▼
        complete bounded behavior model
                         │
       ┌─────────────────┼──────────────────┐
       ▼                 ▼                  ▼
 reference proof   false-positive proof   fairness proof
       │                 │                  │
       └─────────────────┼──────────────────┘
                         │
        noninterference + integrity proofs
                         │
                         ▼
          real counterexample confirmation
                         │
          ┌──────────────┴──────────────┐
          ▼                             ▼
 confirmed issue                  all proofs closed
          │                             │
 remediation + full rerun               ▼
                                  certificate issue
                                        │
                                        ▼
                                     VERIFIED
```

## 22. Product completion gates

The Ray product is complete only when the production CLI proves this entire
flow on real tasks:

1. one real Python task reaches `VERIFIED`;
2. one real Rust task reaches `VERIFIED`;
3. one real C++ task reaches `VERIFIED`;
4. a real missing-test task produces and confirms a false-positive witness;
5. an over-restrictive verifier produces and confirms a fairness witness;
6. an incorrect reference produces and confirms a correctness witness;
7. a test-reading candidate is blocked by noninterference/integrity evidence;
8. a result-forging candidate cannot modify the pass signal;
9. a hidden-state or nondeterministic candidate cannot receive `VERIFIED`;
10. spec/source/test/environment/tool/transcript/certificate tampering is
    detected;
11. unsupported reachable Python, Rust, or C++ behavior returns
    `PROOF BLOCKED`;
12. `ray start` and `ray check` exercise the same complete pipeline;
13. certificate replay succeeds independently; and
14. the clean full test suite and static analysis pass without skipped or
    no-test evidence.

No smoke test, component test, synthetic-only fixture, sampled agreement,
mutation score, PICT report, or partially connected pipeline completes Ray.
