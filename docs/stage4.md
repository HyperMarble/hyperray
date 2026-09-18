# Stage 4 — PROVE

Status: LANGUAGE-NEUTRAL TARGET DESIGN. The current implementation does not provide this full proof path.

## Contract

For every accepted closed boundary, Stage 4 examines every allowed execution for one named requirement.

Stage 4 accepts a model only with a complete semantic coverage certificate. It also accepts one proof query and all referenced artifact identities.

The result is `PROVED` or `DISPROVED`. A model, coverage, certificate, tool, replay, or resource problem returns an engine error, not a verdict.

`PROVED` means that no allowed execution violates the named requirement. `DISPROVED` gives a deterministic witness and the exact cause.

Stage 4 emits `PROVED` only from one of these proof bases:

- An exact fixed-point closure covers every reachable state, and its property scan finds no violation.
- An independent validator accepts an inductive invariant for finite-state safety.
- A proved completeness threshold covers every reachable state and cycle, and the bounded result finds no violation.

Semantic coverage is a separate mandatory gate for all three bases. A proof method cannot replace it.

If none of these proof bases completes, Stage 4 returns an engine error instead of `PROVED`.

## Inputs

The reference model is `M = (S, T, L, G)`. The state set `S` is finite, and `T` contains all program and environment transitions.

The labels `L` identify terminal states, program errors, requirement violations, and monitor acceptance. The root catalog `G` contains all valid entry states.

A proof query is `Q = (q, Iq)`. It selects requirement `q` and only the roots `Iq` that apply to that requirement.

Unrelated function roots do not enter `Iq`. Thus, behavior from an unrelated function cannot disprove the query.

Each selected function instance has exact valid entry states. An impossible precondition requires an accepted proof and does not create a fabricated root.

Unreachable operations remain mapped in the compiler inventory. Query reachability can exclude their states, but it cannot remove their coverage evidence.

## Complete coverage precondition

The coverage validator recalculates the semantic coverage certificate before either proof route starts. It accepts only complete semantic coverage.

The certificate binds these items:

- Every compiler-inventoried operation has a valid disposition and provenance.
- Every mapped operation connects compiler output and machine instructions to transitions.
- Every eliminated operation has an accepted elimination proof and equivalent surviving transitions.
- Every function instance has exact entry information or an accepted impossible-precondition proof.
- Every instruction, environment action, state component, and nondeterministic choice has finite semantics.
- Every artifact reference resolves to its recorded digest.

A count, harness list, loop list, sampled execution, or guessed unwind limit cannot establish complete coverage.

Incomplete or inconsistent coverage causes an engine error. Stage 4 does not calculate reachability or emit a verdict in that case.

## Two proof encodings

Stage 4 has two encodings of the same finite transition system. Both encodings use the same query, labels, observations, and artifact identities.

The explicit finite graph code is the small-model reference. It enumerates states and transitions and gives a direct implementation of the mathematical rules.

The symbolic sequential circuit is the scalable route. It represents state with finite bit vectors and represents choices with finite nondeterministic inputs.

Production proofs use the symbolic circuit route. The explicit graph route serves only as the small-model reference and differential oracle.

For state bits `x`, choice bits `u`, and next-state bits `x'`, the circuit defines `InitQ(x)` and `Next(x, u, x')`.

The circuit also defines every requirement, terminal, error, fairness, and acceptance signal. It does not omit environment or scheduler choices.

BTOR2 is only a transition-system format. It does not establish circuit equivalence, semantic coverage, or a verdict ([Niemetz et al., 2018](https://cs.stanford.edu/~niemetz/publications/2018/NiemetzPreinerWolfBiere-CAV18.pdf)).

The model defines an observation alphabet `A`. The function `obs : (state, transition, state) -> A*` maps each step to its visible events.

An equivalence certificate connects the circuit to the explicit reference semantics. The certificate proves these facts:

- Each state in `Iq` has the matching circuit initial state.
- Each circuit initial state decodes to one state in `Iq`.
- Each reference transition has a matching circuit transition.
- Each circuit transition decodes to a reference transition.
- The encoding preserves labels, observations, acceptance sets, fairness, and terminal status.

The circuit route cannot establish semantic coverage. A failed or missing equivalence certificate causes an engine error.

## Safety fixed point

The least reachable-state fixed point defines reachability for both proof routes. It starts with the query roots:

```text
R0 = Iq
R(n + 1) = Rn ∪ { s' | s ∈ Rn and (s, s') ∈ T }
```

The finite state set makes the sequence stable at some index `k`. The exact reachable set is `R = Rk`.

Induction on path length proves that `R` contains every reachable state. The base case puts every query root in `R0`.

The induction step adds every successor of every known state. Reverse predecessor paths prove that each added state is reachable from a query root.

The graph route enumerates this fixed point. The circuit route represents each image set with formulas over state bits.

Burch et al. define the reachable set as a least fixed point over symbolic relations ([Burch et al., 1992](https://www.cs.cmu.edu/~emc/papers/Conference%20Papers/symbolic%20model%20checking%2010%20%20states%20and%20beyond.pdf)).

Thus, the symbolic route can calculate exact fixed-point sets without an explicit state list.

The circuit route can also use an inductive safety certificate `P`. The certificate validator proves these obligations:

```text
InitQ(x)                 -> P(x)
P(x) and Next(x,u,x')    -> P(x')
P(x)                     -> not Badq(x)
P(x) and Next(x,u,x')    -> not BadTransitionq(x,u,x')
```

These obligations prove by induction that no reachable state or transition violates `q`. They can overapproximate `R` without excluding an allowed execution.

IC3 gives complete finite-state safety through an inductive strengthening or a counterexample ([Bradley, 2011](https://theory.stanford.edu/~arbrad/papers/IC3.pdf)).

Hyperray accepts an IC3 proof only after the independent validator accepts its inductive invariant and solver proof records.

A reachable safety violation returns `DISPROVED`. Its witness identifies the root, path, violating state or transition, named requirement, and cause.

No reachable safety violation returns `PROVED` after proof-certificate validation. The certificate covers every allowed finite execution for the query.

Reachable undefined behavior violates the mandatory safety requirement `NO_UNDEFINED_BEHAVIOR`. Stage 4 cannot hide undefined behavior with a defined-execution scope.

## Temporal and termination proofs

Stage 4 accepts an omega-regular temporal requirement through a generalized Büchi monitor for its negation. Other temporal semantics cause an engine error.

The proof engine forms the product of the machine model and the monitor. It finds reachable components that contain cycles.

An accepting cycle visits every acceptance set and satisfies each declared fairness condition. A one-state component requires a self-loop.

A reachable accepting cycle returns `DISPROVED`. Its witness contains a finite prefix and a repeatable cycle.

If validated fixed-point closure finds no reachable accepting cycle, the temporal requirement gets `PROVED`.

A violating infinite execution in a finite model must repeat a state.

The recurring states form a reachable component. The acceptance visits form an accepting closed walk in that component.

A termination monitor accepts an infinite execution that never enters a terminal state. Thus, a reachable nonterminal accepting cycle disproves termination.

A terminal deadlock trace ends at a nonterminal state that has no outgoing transition. This maximal finite execution also disproves termination.

A terminal-success state is not a deadlock. A trap, abort, or undefined operation keeps its separate label and named safety meaning.

A terminal stutter transition exists only when the query semantics define it. Its terminal label prevents its use as a nontermination witness.

The graph route enumerates reachable components. The circuit route uses exact symbolic fixed-point sets or a proved completeness threshold.

The certificate validator proves all closure, cycle-absence, acceptance, fairness, and threshold obligations against the circuit.

## Deterministic witnesses

Stable root, state, and transition identifiers define one total order. Both proof routes use this order for witness selection.

The safety selector first minimizes the path length. It then selects the least transition-identifier sequence.

The deadlock selector uses the same rule. The temporal selector minimizes prefix length, cycle length, and then the complete identifier sequence.

The temporal selector uses the least rotation of a cycle. Thus, an equivalent cycle has one serialized form.

The graph route uses ordered searches. The circuit route uses fixed-variable lexicographic minimization and records its unsatisfiable minimization proofs.

If deterministic selection exhausts an engine resource, Stage 4 returns an engine error. It does not return a different convenient witness.

`DISPROVED` uses this tagged witness union:

- `safety` contains the root, path, violating state or transition, and cause.
- `temporal_cycle` contains the root, prefix, cycle, acceptance evidence, fairness evidence, and cause.
- `termination_deadlock` contains the root, path, nonterminal deadlock state, and cause.

The validator replays every witness from its selected root. Each step must match the reference transition semantics and recorded observations.

## Proof-certificate validation

An independent validator accepts every positive proof certificate. Prover success text alone cannot produce `PROVED`.

An explicit-graph certificate contains the query roots, reachable states, successor closure, property results, and applicable component data.

The graph validator recalculates each successor and predecessor relation. It also recalculates all violation, terminal, deadlock, acceptance, and fairness labels.

A circuit certificate contains the circuit equivalence proof and each proof record for its invariant, image, closure, or threshold claim.

The circuit validator proves each obligation against the recorded circuit. It also validates the circuit connection to the reference transition system.

A completeness-threshold certificate proves that all reachable states and cycles occur within the threshold. A selected bound alone proves no such fact.

Every certificate names the requirement, boundary, executable image, compiler inventory, semantics, coverage, proof system, and trust identities.

A missing identity, unsupported proof record, invalid obligation, or replay difference causes an engine error.

## Accelerator rules

Kani and other language-specific provers are accelerators only. They are not semantic coverage authorities or independent verdict authorities.

Kani can propose a counterexample, invariant, state partition, or proof. Stage 4 accepts the proposal only through the reference semantics.

A Kani counterexample must replay in the reference model. A positive Kani result requires proof-certificate validation and property-preserving equivalence.

Thus, Kani is an equivalence-checked accelerator. Its harness list and GOTO loop inventory cannot establish compiler, root, environment, or machine coverage.

Rotor is only a bounded, partial RISC-V implementation ([Bolotina et al., 2025](https://arxiv.org/abs/2507.09539)). It cannot establish semantic coverage or an unbounded verdict.

Stage 4 can use Rotor only as an accelerator after circuit equivalence and reference validation.

The same rules apply to every accelerator. Accelerator disagreement, failed replay, or failed equivalence causes an engine error.

## Results and engine errors

`PROVED` records the named requirement and all claim identities. It means that no execution from the query roots violates that requirement.

`DISPROVED` records the same identities, one tagged deterministic witness, and the exact cause.

These conditions cause an engine error:

- The boundary is open or not finite.
- The semantic coverage certificate is incomplete or inconsistent.
- The query root set is missing, fabricated, or not applicable.
- The graph or circuit has an unknown state, transition, instruction, or environment action.
- An equivalence proof, proof certificate, or witness replay fails validation.
- A tool stops, times out, exhausts memory, or produces an unsupported result.

A declared capacity result is program behavior in the transition system. Proof-engine resource exhaustion is an engine error.

Stage 4 never converts an engine error into `PROVED` or `DISPROVED`.

## Implementation status

This document specifies the accepted Stage 4 architecture. The current implementation does not satisfy the full graph, circuit, certificate, and integration contract.

No current Rust or multi-language result has complete semantic coverage through this Stage 4 design. Completion requires all applicable project gates to pass.
