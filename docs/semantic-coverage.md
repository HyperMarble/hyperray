# Semantic Coverage Product Gate

Status: LANGUAGE-NEUTRAL TARGET DESIGN. The current implementation does not provide this full coverage product.

## Purpose

Semantic coverage states that the transition model represents the full declared boundary. It does not state which model states are reachable.

The coverage product joins generated catalogs with exact set equalities. An independent validator accepts this product before proof work starts.

One missing or extra case is an engine error. Stage 4 cannot start, and Hyperray does not emit a logic verdict.

## Separation from proof

Semantic coverage, query reachability, and a logic verdict are three separate results.

Semantic coverage compares source, executable, semantic, circuit, root, and provenance sets. It applies before a query selects its reachable states.

Reachability starts from the query roots only after coverage succeeds. The verdict then uses the reachable fixed point or another accepted proof base.

A proof result cannot repair or replace a coverage error. A counterexample also cannot make an incomplete model complete.

## Inputs and identities

The product uses these recorded inputs:

- The source tree and the closed finite boundary.
- The selected compiler, linker, runtime, target, ABI, and executable image.
- The machine profile, ISA semantics, EEI semantics, memory model, scheduler, and environment model.
- The generated catalogs, provenance edges, selected route model, and equality proof records.
- The requirement, query, root-selection rule, and observation function.

Each input has a stable identifier and a content digest. A stale, missing, ambiguous, or duplicate identity is an engine error.

## Generated catalogs

Each catalog generator reads a recorded artifact and emits stable identifiers in canonical order. It also records its version and all input digests.

Each equality compares an independent producer catalog with its consumer inventory. One component cannot define both sides from the same output.

### Compiler operations

The compiler-operation catalog contains every concrete operation in the source and linked runtime. It also contains compiler-generated initialization, cleanup, and dispatch operations.

Each operation has one base disposition:

- `mapped` identifies compiler output, executable instructions, semantic cases, and transition evidence.
- `eliminated_with_proof` identifies an accepted elimination proof and the equivalent surviving behavior.

For a query, `unreachable_with_proof` can annotate a `mapped` operation. This annotation keeps every mapping and adds only a reachability proof.

An operation cannot disappear because of optimization or unreachable code. An omitted operation is a coverage error.

### Executable instructions

The executable-instruction catalog comes from the exact executable image and target decoder. It includes runtime, startup, linker-generated, and compiler-generated instructions.

Each instruction record contains its image address, bytes, decode, owning image region, and provenance edges.

An instruction identifier includes the executable digest and address. Thus, instructions from two images cannot merge.

Unknown bytes or an ambiguous decode cause an engine error. The validator never assigns a convenient instruction meaning.

### ISA and EEI cases

The ISA/EEI semantic-case catalog contains every applicable machine and environment case for the boundary.

The ISA part contains normal results, traps, faults, privilege effects, memory effects, and all operand-dependent branches for each decoded instruction.

The EEI part contains system calls, external events, capacities, errors, signals, time, random data, files, memory mappings, and thread actions.

The concurrency cases include scheduler choices, thread creation, joining, cancellation, atomics, data races, fairness, and progress.

The memory cases include allocation, free, address reuse, provenance, bounds, permissions, and uninitialized data. Runtime cases include stack exhaustion and garbage collection.

Every nondeterministic result comes from a declared finite set. An unknown or unrestricted result makes the boundary open and causes an engine error.

### Circuit regions

The circuit-region catalog covers the generated next-state relation with named semantic regions. Each region has a guard, updates, labels, choices, and provenance.

The catalog includes initialization, machine steps, environment steps, scheduler steps, monitor steps, traps, and declared terminal stutter.

Separate circuit outputs identify terminal states, deadlocks, errors, observations, fairness, and monitor acceptance.

An overlapping region is valid only when all overlaps encode the same reference transitions and labels. The equivalence proof records this fact.

An orphan region adds behavior and causes an engine error. A missing region removes behavior and causes an engine error.

### Roots

The root catalog contains every program entry and every in-scope function instance. It derives entries from compiler metadata, the ABI, and the boundary.

Each function instance has its exact valid standalone entry states. An impossible precondition has an accepted proof instead of a fabricated empty root.

For requirement `q`, the query rule generates `Iq` from the applicable catalog entries. It excludes unrelated entries without removing their coverage evidence.

A handwritten root list cannot establish coverage. A fixture can measure the generator, but it cannot become proof evidence for another artifact.

### Provenance

The provenance catalog gives the complete forward and reverse chain:

```text
compiler operation -> compiler output record -> executable instruction
linker or startup origin -> executable instruction
executable instruction -> ISA semantic case
declared EEI action -> EEI semantic case
requirement or model rule -> synthetic semantic case
semantic case -> explicit transition or symbolic circuit region
```

Each edge records both endpoint identifiers, the producer, the input digests, and the applicable elimination or equivalence proof.

Every explicit transition and circuit region maps back to an instruction, environment action, or declared synthetic operation.

A missing reverse edge creates an orphan model action. A missing forward edge removes compiled behavior. Both conditions cause an engine error.

## Exact set equalities

The validator compares identifiers and relations, not only counts. Equal counts with different members do not satisfy the gate.

Use these sets:

- `O` is the set of compiler-operation identifiers.
- `Om` is the set of `mapped` operations.
- `Oe` is the set of `eliminated_with_proof` operations.
- `B` is the set of compiler-output records.
- `X` is the set of linker and startup origin records.
- `I` is the set of executable-instruction identifiers.
- `E` is the set of required EEI actions.
- `Kisa` and `Keei` are the sets of generated semantic-case identifiers.
- `Ksynthetic` is the set of monitor and declared synthetic cases.
- `S` is the finite state set of the reference model.
- `Tg` is the explicit graph transition relation.
- `Rc` is the set of symbolic circuit regions.
- `G` is the generated root catalog.
- `L` is the required label function.
- `obs` is the required observation function.

The product requires these exact set equalities:

```text
O = Om ⊎ Oe
domain(operation_to_output) = Om
range(operation_to_output) = B
required_noncompiler_origins(linker, executable) = X
domain(origin_to_instruction) = B ⊎ X
range(origin_to_instruction) = I
decoded_instructions(executable, target) = I
domain(instruction_to_ISA_case) = I
range(instruction_to_ISA_case) = Kisa
required_ISA_cases(I, target) = implemented_ISA_cases(machine) = Kisa
required_EEI_actions(executable, boundary) = E
domain(EEI_action_to_case) = E
range(EEI_action_to_case) = Keei
required_EEI_cases(E, boundary, environment_profile) = implemented_EEI_cases(environment) = Keei
required_synthetic_cases(requirement, boundary) = implemented_synthetic_cases(model) = Ksynthetic
required_roots(function_instances(O), compiler, ABI, boundary) = implemented_roots(model) = G
query_roots(q, G) = { r in G | applies(q, r) }
required_common_provenance(O, B, X, I, E, Kisa, Keei, Ksynthetic) = recorded_common_provenance
required_graph_provenance(Kisa, Keei, Ksynthetic, Tg) = recorded_graph_provenance
required_circuit_provenance(Kisa, Keei, Ksynthetic, Rc) = recorded_circuit_provenance
reverse(recorded_forward_common_provenance) = recorded_reverse_common_provenance
reverse(recorded_forward_graph_provenance) = recorded_reverse_graph_provenance
reverse(recorded_forward_circuit_provenance) = recorded_reverse_circuit_provenance
```

The symbol `⊎` means a disjoint union. Each equality rejects missing members, extra members, and duplicate dispositions.

The graph certificate uses the graph-provenance equality. The circuit certificate uses the circuit-provenance equality and the relation-equivalence proof.

An eliminated operation also requires equality between its removed behavior and the named surviving behavior. The accepted elimination proof establishes this equality.

For each query, `unreachable_with_proof(q) ⊆ Om`. This annotation does not change `O`, `I`, `Kisa`, `Keei`, or their provenance relations.

## Explicit finite graph route

The explicit finite graph is the small-model reference. Its transition generator instantiates every applicable ISA, EEI, scheduler, and synthetic case.

Let `Cases = Kisa ⊎ Keei ⊎ Ksynthetic`. The graph relation satisfies this equality:

```text
Tg = { instantiate(k, s) | k in Cases, s in S, guard(k, s) }
```

The generator records a no-instance proof for a case with an unsatisfied guard. It cannot silently omit that case.

Each transition in `Tg` has one or more semantic-case sources. Each applicable semantic-case instance contributes its exact transitions to `Tg`.

The validator regenerates successors for every state in the small reference model. It compares exact transition identifiers, target states, effects, observations, and labels.

Reachability does not take part in this equality. Unreachable transitions remain in the complete graph and its provenance map.

## Symbolic sequential circuit route

The symbolic sequential circuit is the production route. Its state bits, input bits, initialization predicate, and next-state relation encode the same reference semantics.

Each circuit region comes from a generated semantic case. The circuit contains no handwritten default branch for an unknown action.

Let `encode` and `decode` be the recorded state maps. The circuit route satisfies these relation equalities:

```text
decode({ x | InitQ(x) }) = Iq
decode({ (x, u, x') | Next(x, u, x') }) = Tg
decode_labels(circuit_outputs) = L
decode_observations(circuit_outputs) = obs
regions(Next) = Rc
domain(case_to_region) = Cases
range(case_to_region) = Rc
```

The first equality applies separately to each query. The other equalities include state changes, choices, errors, observations, acceptance, fairness, and terminal labels.

The second equality is mathematical. The production validator proves it by semantic case without materializing the explicit relation `Tg`.

The complete translator and certificate method are specified in
[`instruction-translation.md`](instruction-translation.md).

The forward proof shows that each reference transition has a circuit transition. The reverse proof shows that each circuit transition has a reference transition.

The circuit equality is a proof obligation, not a differential-test result. Small-model differential tests can find faults, but they cannot establish coverage.

BTOR2 can store the circuit relation, but the format does not prove these equalities. A solver result also cannot prove semantic coverage.

## Product decision

The complete coverage product is this conjunction:

```text
CompleteCoverage =
  OperationEquality
  and InstructionEquality
  and SemanticCaseEquality
  and RootEquality
  and ProvenanceEquality
  and RouteEquality
  and ArtifactIdentityEquality
```

The independent validator recalculates each member or validates its accepted proof certificate. A producer cannot accept its own unexamined claim.

The validator emits one completed coverage certificate only when every equality succeeds. The certificate binds the selected graph or circuit route.

One missing or extra operation, instruction, semantic case, root, provenance edge, transition, or circuit region causes an engine error.

A handwritten fixture list, harness list, loop inventory, test result, coverage percentage, or sampled trace is not semantic coverage evidence.

Kani, Rotor, BtorMC, and other accelerators cannot accept this product. They can supply proposals that pass the same independent validation.

## Trust and status

The source-level claim trusts the compiler and linker unless a trace-preservation proof removes that trust. The coverage product records this trust condition.

The binary-level claim depends on the decoder, ISA and EEI semantics, circuit encoder, coverage validator, and proof-certificate validator.

This document specifies the target coverage product. Existing generated files or passing fixtures do not by themselves satisfy this contract.
