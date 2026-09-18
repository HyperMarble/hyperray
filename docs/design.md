# Hyperray design

Status: ACCEPTED TARGET ARCHITECTURE 2026-09-04. The implementation is not
complete.

This file replaces the former Stage 3 proof design. The detailed reference
proof is in `docs/proof-machine.md`.

`docs/specs/evidence-rule.md` remains in force. The language adapter documents
retain measured tool facts, but they do not define proof coverage.

## 1. What Hyperray does

Hyperray receives the base source, a solution patch, task text, named
requirements, and a closed finite boundary.

The task text supplies the contract. Each formal requirement links to its
contract source and has finite-state semantics.

The source and runtime supply the implementation. The finite boundary supplies
the exact proof scope.

For each named requirement in an accepted closed boundary, Hyperray returns one
of two logic verdicts:

- `PROVED` means that no allowed execution violates the requirement.
- `DISPROVED` includes the requirement, applicable state, path, and cause.

A model, coverage, compiler, tool, or resource problem returns an engine error.
An engine error is not a logic verdict.

After the proof, Hyperray can derive test rows and emit `instruction.md` with a
test file. These artifacts do not strengthen the proof verdict.

## 2. Seven stages

The seven stages use this route:

```text
base + solution.patch + task text + finite boundary
  -> [1] EXTRACT   manifest.json       changed functions placed by the compiler
  -> [2] SHAPE     findings.json       target-policy lint findings
  -> [3] MODEL     model + coverage    executable image, provenance, transitions
  -> [4] PROVE     result.json         fixed-point and accepting-cycle result
  -> [5] ADEQUACY  adequacy.json       mutation evidence for test rows
  -> [6] PLAN      plan.json           contract-backed test rows
  -> [7] EMIT      instruction + tests one ordered source of emitted rows
```

Stages 1 and 2 give source reports. They do not establish semantic coverage or
a logic verdict.

Stages 3 and 4 implement the reference proof route. Stages 5 through 7 create
test artifacts after that route.

## 3. Semantic coverage, Reachability, and Verdict

These values are separate. No tool output can substitute one value for
another.

### Semantic coverage

Semantic coverage states whether the finite transition model represents every
declared operation and environment action.

Every compiler-inventoried operation remains in provenance. Each surviving
operation maps through compiler output and machine instructions to model
transitions.

An eliminated operation requires a checked proof to equivalent surviving
transitions. Every executable instruction maps back to an inventoried or
declared synthetic operation.

Coverage records `mapped` or `eliminated_with_proof`. A query can refine a
mapped operation to `unreachable_with_proof` without removing its mapping.

Every in-scope function instance has exact standalone entry information. A
query selects only the roots that apply to its named requirement.

A function normally has a nonempty valid entry set. An impossible precondition
requires a checked proof instead of a fabricated or silently empty root.

Unreachable operations stay in the coverage map. Reachability cannot remove
them from the coverage denominator.

Only a complete semantic coverage certificate can enter Stage 4. An incomplete
certificate returns an engine error.

### Reachability

Reachability starts from the roots that the named requirement selects. It is
the least fixed point of the complete transition relation.

Reachability marks mapped transitions that occur on allowed executions. It
does not decide whether an unmapped item is safe.

### Verdict

A verdict answers one named requirement over the reachable states and
transitions. `PROVED` and `DISPROVED` are the only logic verdicts.

`PROVED` requires complete semantic coverage and a completed proof.
`DISPROVED` requires a replayable violating execution in the reference model.

## 4. Rules for every stage

Every stage obeys these rules:

- Tools supply code facts. Hyperray does not guess language syntax or tool
  behavior.
- Every boundary value names its source. Hyperray does not invent a proof
  limit.
- Compilation, semantic coverage, reachability, and verdict remain separate
  result fields.
- A `DISPROVED` result means that code behavior violates a named requirement.
- An unsupported construct, incomplete artifact, timeout, or malformed result
  is an engine error.
- A stage never changes an engine error into a verdict.
- The contract gives each requirement. The code gives its exact mechanism and
  finite shape.
- A row learned only by operating the reference implementation is not contract
  evidence.
- Each tool invocation records the executable identity, version, arguments,
  inputs, and outputs.
- Fixtures stay outside adapter logic. No adapter contains a fixture name.

The compiler, linker, machine semantics, environment semantics, coverage
checker, and proof-certificate checker form the stated trust boundary. Section
11 gives the detailed rule.

## 5. Stage 1 — EXTRACT

Stage 1 reports every function that the patch changes. It does not define the
operation inventory for semantic coverage.

The first pass reads only the patch text. It records files, hunks, changed
ranges, and function names that a hunk defines.

The source locator opens only patch-named files. It records whole function spans
and the reason for each file read.

The compiler pass reads item names, parents, spans, values, input types, and MIR
bodies. The current schema is analysis input, not complete proof semantics.

The current adapter joins functions by path, span containment, and name. It
keeps the compiler item path in the joined row.

For Rust, the current route uses the pinned `rustc_public` driver. The driver
uses a fresh wrapper path and a Hyperray-owned target directory.

Existing project artifacts cannot supply a cached result from a different
compiler invocation. The source locator records every file it opens and its
reason.

The source manifest keeps these facts:

```text
function: path, source name, start line, end line, text
hunk:     path and line for a hunk outside all functions
opened:   path and reason
```

The compiler join adds `item_path` and one status. The current status values are
`Extracted`, `Missing`, and `FileNotSeen`.

The Stage 1 acceptance tests compare patch hunks with compiler joins. They also
examine file access and stale-artifact isolation.

These facts preserve the existing Rust extraction design. They do not claim
that the current driver inventories every runtime operation.

## 6. Stage 2 — SHAPE

Stage 2 reports the target repository lint policy. It does not rewrite source
and does not create `shaped.patch`.

For Rust, Cargo and Clippy read the target crate lint levels, source attributes,
and Clippy configuration. Hyperray does not inject its own lint policy.

The adapter Clippy configuration applies only to Hyperray source. It never
changes the target policy.

Cargo JSON output supplies each finding. A finding contains the lint, path,
primary span, and original message.

Style is not proof evidence. A long or unconventional function continues to
the same compiler and proof stages.

The Stage 2 acceptance tests require these facts:

- A target without a lint policy does not receive the Hyperray policy.
- Each retained finding has a Clippy name and a primary source span.
- Each finding in a changed function stays inside that function span.

These facts preserve the existing Rust Stage 2 behavior. They do not establish
semantic coverage.

## 7. Stage 3 — MODEL

Stage 3 consumes the compiler inventory, executable, finite boundary,
requirements, and build identity.

The stage produces these connected artifacts:

- The compiler operation inventory
- The compiler operation catalog, executable image, and provenance
- The typed machine semantic IR and finite external contracts
- The root catalog and each query-specific root set
- The complete semantic coverage certificate.

The transition model includes machine state and each declared external-contract
state. Unknown semantics cause an engine error.

Rust, C, C++, and Go compile into the same machine model. The Python route
includes the pinned interpreter and bytecode.

The first machine adapter uses a static RV64 executable and pinned Sail
semantics. Compiler-level adapters can accelerate analysis only after they are
proved equivalent to this machine route.

Kani and other language-specific provers are accelerators only. A Kani harness
or GOTO loop inventory is not a coverage authority.

Stage 3 keeps unreachable compiler operations mapped. It does not classify an
operation as absent because no query root reaches it.

An independent checker must accept the coverage certificate. Otherwise,
Stage 3 returns an engine error.

The former Kani and CBMC loop-inventory design is replaced. `docs/stage3.md`
records the replacement and the current gaps.

## 8. Stage 4 — PROVE

Stage 4 accepts only a model with a complete semantic coverage certificate. It
first calculates the least reachable-state fixed point.

For a safety requirement, Stage 4 examines every reachable state and
transition. A reachable violation returns `DISPROVED` with an exact replay
path.

An omega-regular temporal requirement uses a generalized Büchi monitor for its
negation. Stage 4 examines all reachable accepting cycles in the product.

A reachable accepting cycle returns `DISPROVED` with a prefix and repeatable
cycle. A nonterminal deadlock is a separate finite termination violation.

The `DISPROVED` witness is a tagged union. Safety, temporal-cycle, and
termination-deadlock witnesses keep their different state and path forms.

The finite-state induction and accepting-cycle arguments are in
`docs/proof-machine.md`. A sampled run or guessed unwind limit cannot replace
them.

The proof result includes the requirement, boundary, executable image,
inventory, semantics, coverage, proof, and trust identities. Each requirement
gets a separate verdict.

Any proof-model, certificate, tool, replay, or resource problem returns an
engine error. Stage 4 does not emit a verdict for that requirement.

## 9. Stage 5 — ADEQUACY

Stage 5 measures whether the generated tests notice changes to patched lines.
Mutation results are test evidence, not semantic coverage.

A surviving mutant can identify a missing contract-backed test row. It cannot
invalidate or strengthen a machine proof by itself.

If a survivor exposes an omitted contract requirement, Stage 4 proves that
named requirement before later stages use its verdict.

An equivalent mutant stays recorded as equivalent. A tool or resource problem
returns an engine error instead of a mutation judgment.

## 10. Stages 6 and 7 — PLAN and EMIT

Stage 6 partitions contract-backed rows from proof and adequacy artifacts. Each
row names its requirement and evidence source.

Stage 7 writes `instruction.md` and the test file from the same ordered row
list. Both outputs keep the same row identifiers and order.

Generated tests can replay `DISPROVED` paths and protect required behavior.
Passing tests do not change `PROVED` and do not establish coverage.

## 11. Language routes and trust

Language-specific source stays in five separate directories:

```text
adapters/rust/
adapters/c/
adapters/cpp/
adapters/go/
adapters/python/
```

An adapter identifies source operations, invokes its language toolchain, and
records provenance. It does not contain machine semantics, circuit rules,
coverage policy, or proof logic. Those shared parts stay in `machine/`,
`semantic/`, `circuit/`, `coverage/`, and `proof/`.

C and C++ can use the same compiler infrastructure, but their adapters do not
share production source files. This separation keeps language rules and
failure reports local to the language that owns them.

The adapter binary is program-independent. A new source program, executable,
or finite boundary changes generated artifacts only. It does not require a
fixture rule, a source-pattern rule, or a Hyperray source change.

The selected build identity includes the compiler, linker, flags, target,
libraries, runtime, and artifact digests. Each different identity gets a
different proof.

The binary-level claim uses the recorded executable image and target instruction
semantics. Without checked trace preservation, source fidelity is conditional
on compiler and linker trust.

The environment claim trusts the declared machine, memory, scheduler, and
external-service semantics. Behavior outside that boundary is outside the
claim.

The coverage checker and proof-certificate checker are trusted. The application
and runtime behavior are not assumed correct because their transitions are
explored.

An accelerator result requires equivalence with the reference transitions or
replay in them. The accelerator does not become a coverage authority.

Each source-language query includes `NO_UNDEFINED_BEHAVIOR`. Hyperray must prove
this requirement before another source-level `PROVED` result.

A component summary requires contextual trace equality with its reference
component. Without that checked theorem, Hyperray uses the monolithic model or
returns an engine error.

## 12. Implementation status

The current implementation does not satisfy this entire architecture. This
document makes no complete-coverage claim for Rust or another language.

Stage 1 Rust extraction and Stage 2 Rust lint reporting have implemented
components. Their local facts do not imply proof completion.

Stage 3 has status `REPLACED` and `NOT COMPLETE`. Its former Kani/GOTO coverage
route cannot issue a semantic coverage certificate.

The gates under `gates/` define implementation completion. Only measured gate
evidence can change this status.
