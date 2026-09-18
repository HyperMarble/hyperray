# Generated-model proof route

Status: proof integration in progress. This route does not establish full coverage.

## Contract

Sail supplies instruction behavior. Its compiler supplies Lean definitions.
Hyperray must not replace these definitions with instruction-name rules.
Lean validates proof terms about the imported definitions.
The existing finite-boundary and coverage requirements in `PLAN.md` remain unchanged.

A successful model build establishes type correctness, not translation equality.
A proof about one execution does not cover all permitted executions.
Every accepted theorem must name the model, boundary, requirement, and dependencies.
Unproved platform operations must remain visible proof obligations.

## First reusable obligation

Prove memory identities directly against the pinned Sail support library.
The statements quantify over addresses, stored values, and prior memory states.
They must not contain Rust fixtures, instruction encodings, or program addresses.
These lemmas support the memory connection. They do not establish that connection alone.

The proof must preserve ordinary write effects and reads at other addresses.
A read outside initialized memory must retain its declared error.
The proof must not add assumptions that silently remove these executions.

## Imported model audit

The generated model revision is `51c635c460125ac87955d061fcd902ce03b8011f`.
Its Sail support revision is `079463134b9c50450b8393e1566a09fc492a34d9`.
The local Lean release is 4.29.0.

The generated `SailM` currently selects `trivialChoiceSource`.
That source selects fixed values for undefined primitives.
The support library gives barriers no state effect.
The generated extras declare platform and floating-point operations as axioms.
Thus, this default model is not an all-choice, concurrent coverage certificate.
The proof integration must account for these differences before acceptance.

`docs/sail-event-route.md` records the alternative event-interface audit.
Its imported primitives preserve finite choices and barriers, but its range primitive omits an endpoint.
The pinned RISC-V source also requires a different memory interface.
The audit does not authorize a replacement of the current model.

## Measured evidence

The complete imported model passed `lake build` with 135 jobs.
Lean accepted the three universal memory theorems in `proof/sail_memory/lean/`.
Their guarded dependencies contain only `propext`, `Classical.choice`, and `Quot.sound`.
Lean rejected the changed read-after-write claim in `fixtures/lean/changed-read.lean`.

`proof/sail_model/lean/AxiomAudit.lean` imports the whole generated model.
Lean reports platform, reservation, random-input, and floating-point axioms
for both `execute` and `try_step`.
Thus, the machine-step audit does not establish an unconditional coverage proof.

## Completion

Importing the model and proving memory identities are intermediate obligations.
The final proof must preserve initialization, transitions, errors, observations,
termination, and all external choices within the declared finite boundary.
Both missing behaviors and added behaviors invalidate coverage.
Only the existing full coverage contract can authorize a final proof verdict.
