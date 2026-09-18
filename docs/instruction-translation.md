# Machine Translation Proof

Status: TARGET DESIGN. The complete translator and proofs do not exist yet.

## Current measured slice

The current proof slice takes four official operation catalogs and the
official `ADDIW` operation. It takes their rules from one pinned Sail RISC-V
revision. Its Lean theorems cover all 22 operations and all values in their
finite RV64 input types. One-bit changes in all five families produce
counterexamples.

The typed-tree catalog tool found 23 families in the instruction declaration,
decoder, and execution sets. The exact set validator accepted these equal
sets. The family-local meaning proof covers five families: 5/23, or 21.7%.

The pinned Sail C backend also lowered the complete profile to 1,735 JIB
definitions. The traversal recorded 167,953 constructor origins. It found 14
instruction kinds, 8 value kinds, 7 operation kinds, 3 storage kinds, and 16
type kinds.

Sail gave two incomplete-pattern warnings. The lowered model contains the
`undefined` instruction kind. Later coverage must map this kind to explicit
behavior. It must not omit the kind.

This result is not complete machine coverage. The larger `I_insts` Sail model
generated Lean source, but Lean rejected that generated project. Hyperray does
not use that output as proof evidence.

## Claim

For the accepted computer profile, Hyperray must prove this equality:

```text
executable instruction behavior = semantic IR behavior = circuit behavior
```

The executable supplies instruction bytes and addresses. Pinned Sail supplies
their reference meaning. Sail lowers the complete model to JIB. The circuit
supplies the proof model.

## Translation route

Sail lowers its typed tree to JIB, its compiler IR. JIB is the Hyperray
semantic IR for the machine model. Hyperray does not contain an instruction
name table or a handwritten rule for each instruction.

Hyperray builds its census with the pinned upstream `c_backend.ml` source in a
temporary directory. It does not copy or replace the Sail translator.

The production route uses the official Sail SystemVerilog backend as the JIB
lowerer. Hyperray does not write instruction equations or a second JIB parser.
A small plug-in selects the typed-tree roots from the machine profile. The
root, integer width, and bit-vector width are proof inputs.

The Sail backend emits the SystemVerilog circuit. A pinned synthesis tool emits
BTOR2 from that circuit. Each unsupported or nonfinite construct returns an
engine error.

The width inputs need proved range obligations. Hyperray never selects a width
because it worked for one fixture.

The accepted JIB profile contains finite bit vectors, arrays, state reads,
state writes, guards, choices, traps, observations, and external-contract
calls. One Lean evaluator gives JIB its mathematical meaning. The official
Sail backend creates the circuit from the same JIB program.

Lean proves that JIB evaluation and SystemVerilog circuit evaluation are equal
for each JIB constructor. The proof applies to all instructions that use that
constructor. It does not contain an instruction family name.

The coverage product records the pinned Sail compiler and SystemVerilog backend
as part of the trust base. Later translation theorems can remove this trust.

The existing instruction-family proofs are reference fixtures. They measure
the generated route and its mutations. They are not the production translator.

## Program independence

The production route takes the executable, boundary, machine profile,
external contracts, and requirement as data. It must not contain a source
function name, fixture name, program address, program byte sequence, or chosen
bound.

A source change produces a new executable and new generated catalogs. It does
not require a Hyperray code change. The same JIB semantics, circuit lowerer,
and proof rules apply to the new finite program.

The machine profile is fixed for one proof. Its complete instruction semantics
come from the pinned Sail model. A different accepted profile uses its own
pinned model and the same generic JIB route.

## Coverage certificate

The certificate compares independently generated sets:

```text
decoded executable instructions
= required Sail cases
= translated semantic IR cases
= proved circuit regions
```

The comparison works in both directions. It rejects a missing member, an extra
member, a duplicate owner, a stale digest, or a failed theorem.

The JIB-to-circuit gate uses three independent catalogs. The Sail traversal
emits every JIB origin. The lowerer emits one translation for each origin. The
circuit builder emits every circuit region.

Each translation records its lowerer rule, proof obligation, disposition, and
circuit regions. An emitted translation has one or more regions. A proved
erasure has no region. The validator requires these exact equalities:

```text
JIB origin identifiers = translated origin identifiers
circuit region identifiers = translated region identifiers
```

A circuit region has one owner because one translation contains its identifier.
A duplicate owner is an error. A count match cannot replace either equality.

Async functions, generic instances, dynamic dispatch, recursion, and threads
compile or execute through this same machine route. External calls use named
finite contracts. Hyperray does not model the operating-system implementation
behind those contracts.

## Completion rule

Differential tests and solver results can find defects. They cannot close this
gate. The gate closes only when the proof validator accepts every required
theorem and exact catalog equality accepts every required case.

A changed instruction equation must make its theorem fail. A missing or extra
case must make coverage fail. Hyperray emits no logic verdict after either
failure.
