# Sail and Isla Integration

Status: MEASURED SAME-PROGRAM ISLA SLICE. Independent circuit translation is in progress.

## Purpose

Hyperray uses Sail as the source of machine instruction semantics. It uses Isla to translate these semantics into solver formulas.

Hyperray does not contain a rule for each instruction. A new input program does not require a Hyperray source change.

## Route

```text
bounded program
  -> compiler and linker
  -> executable instructions
  -> pinned Sail machine model
  -> Isla symbolic execution
  -> SMT constraints and machine events
  -> solver result
  -> Hyperray evidence and coverage gate
```

Sail supplies the instruction behavior. Isla supplies path exploration, machine events, and SMT constraints.

Hyperray supplies the finite boundary, the property, artifact identity, coverage evidence, and the final result.

## Public operation

The machine integration accepts one request with these inputs:

- The Isla executable.
- The Sail model snapshot.
- The Isla configuration.
- The memory model.
- The bounded program.
- The program-counter visit limit.
- The time limit.
- The maximum output size.

Each file has an expected SHA-256 digest. A changed or missing file causes an engine error.

The request contains the negation of the required property. An allowed execution is a counterexample.

A forbidden execution means that Isla found no counterexample in the accepted bounded model. This result remains a proposal until coverage accepts it.

## Result

The integration returns one of these values:

- `counterexample_found` with the solver state.
- `no_counterexample_found` with the candidate counts.
- An engine error with the exact cause.

The result records the tool version, tool digest, input digests, bounds, raw-output digest, and elapsed time.

The integration never changes an engine error into a proof result. A timeout, process error, parse error, or visit-limit error stops the operation.

## Coverage

The Isla result does not establish semantic coverage by itself.
Static instruction coverage and query execution are separate records:

```text
loaded program instructions = instruction-footprint records
executed query instructions are a subset of loaded program instructions
```

The static comparison works in both directions. A missing, extra, changed,
duplicate, or stale footprint causes an engine error.

Every executed address and encoding must match a loaded instruction.
The execution report must include the program entry instruction.
Duplicate execution-inventory records cause an engine error.

Loaded instructions absent from the query stay in the static inventory.
The result lists them as `not_observed`, not `unreachable_with_proof`.
Their absence does not establish an independent reachability proof.

The trusted Isla route depends on the pinned executor for path completeness.
An inventory comparison alone cannot prove path completeness or behavioral equivalence.

An unavailable primitive is permitted only with a proof that the bounded program cannot reach it. Otherwise, it causes an engine error.

## Instruction trace route

Hyperray sends each loaded instruction's bytes and address to `isla-footprint`.

Isla decodes the bytes with the pinned Sail model. Hyperray does not decode instruction names or contain opcode rules.

Each successful operation must return one or more semantic traces. Hyperray records the instruction address, bytes, trace count, and raw trace digest.

The engine, Sail snapshot, and Isla configuration must come from one recorded release set. A mixed release set causes an engine error.

### Release manifest

One JSON manifest binds these values:

- Release identifier.
- Isla version and executable SHA-256 digest.
- Sail snapshot SHA-256 digest.
- Isla configuration SHA-256 digest.

The request accepts a release only when every measured value equals its manifest value.

The manifest is also an identified artifact. Deployment policy supplies its trusted digest.

An arbitrary self-declared manifest records inputs but does not establish trusted provenance.

### Diagnostic dispositions

Isla reports every unavailable primitive while it loads the model. The primitive remains a normal call inside its instruction semantics.

If execution reaches that call, Isla returns a missing-function error. Hyperray accepts no instruction report after that error.

A successful complete instruction run therefore records each unavailable primitive as not called in that run.

Each disposition contains the exact diagnostic and complete output digest. The coverage validator compares every diagnostic and disposition in both directions.

Any other diagnostic stops the operation. A release configuration must remove stale entries instead of hiding their warnings.

The instruction inventory and trace inventory must contain the same addresses and bytes. Missing, extra, changed, or duplicate records cause an engine error.

An unmatched RISC-V encoding maps to Sail's declared illegal-instruction case. It is a modeled trap, not an omitted instruction.

This route establishes instruction-to-Sail coverage. It does not establish Sail-to-circuit equality or complete environment behavior.

## Semantic trace artifact

The next route keeps the exact Isla standard output for each instruction. It does not use trace simplification or hidden-event options.

Isla output already contains SMT definitions and generic state events. Hyperray will translate this fixed event grammar instead of instruction rules.

The pinned Isla executor returns after all path fractions total one. The footprint command returns an error if a reachable path fails.

The coverage validator recalculates the digest from the retained output and diagnostics. A changed trace cannot enter circuit translation.

The first release includes the pinned Isla executor in the trust base. A later proof can reduce this trusted base.

```text
Sail behavior -> complete Isla event traces -> circuit behavior
```

The remaining proof maps each accepted Isla event kind to its circuit state effect. No rule names a source function or RISC-V instruction.

## Program independence

Production code must not contain fixture names, function names, instruction bytes, addresses, or property values.

Tests use different programs with the same public operation. A program change changes input artifacts, not Hyperray source.

### Direct Rust compiler integration test

The test invokes `rustc` for the selected RISC-V target and retains its object.
The linker produces an ELF with an explicit entry and return boundary.
The existing `BuildProgram` and `VerifyProgram` APIs consume that ELF.
No Rust MIR pattern analysis enters this path.

The test queries different inputs of a branch and different source programs.
It requires both correct-property acceptance and a false-property counterexample.
It also requires static coverage to retain instructions absent from a query.
These are bounded machine queries, not source-language coverage certificates.

The direct test passed on 2026-09-05 with rustc 1.98.0 and LLD 22.1.8.
It compiled `fixtures/rust/machine/branch.rs` and `arithmetic.rs` directly.
Both branch inputs and the arithmetic input returned `PROVED` for their stated results.
Each changed result returned `DISPROVED` with a counterexample state.

| Query | Static instructions | Observed | Not observed |
|---|---:|---:|---:|
| Branch, input 0 | 6 | 3 | 3 |
| Branch, input 8 | 6 | 4 | 2 |
| Arithmetic, input 3 | 4 | 4 | 0 |

The return register points to the declared empty harness boundary.
These fixtures do not invoke an operating system or prove arbitrary input domains.

## Symbolic input and candidate evidence

A register absent from the initial-value table uses the pinned model defaults.
An uninitialized register becomes a symbolic value when Isla reads it.
This behavior depends on the model and configuration, not on omission alone.
The symbolic Rust test requires a symbolic read of the 64-bit input register.

Mixed path results require candidate labels. A count cannot identify a witness.
The local `candidate-status-v1` tool patch adds optional labels to Herd state rows.
It also resolves symbolic final-register values through Isla's existing model reader.
The solver query requests each distinct symbolic final-register value.
It does not change instruction semantics, constraints, or solver decisions.

Hyperray accepts a mixed result only when every state has an explicit status.
The row counts must equal the reported positive and negative counts.
An allowed row must contain a concrete state, not a solver variable name.
Legacy unlabeled witnesses remain valid only when every candidate is allowed.
Negative-only results require a forbidden row for every candidate.
The tool version and executable digests identify the patched release.

The symbolic test passed on 2026-09-05 with both compiled branch paths.
All six static instructions appeared in the semantic inventory.
The solver rejected output `65` and produced concrete outputs `17` and `64`.
The test requires these exact final-register values.
These final states do not contain replayable input traces.
The patch, build commands, and measured digests are in `tools/isla/`.

## Measured proof slice

The local proof built the official Sail-to-Isla plug-in and the Isla engine. Sail generated a 15,663,211-byte RISC-V IR file.

For `addi x5,x0,3`, Isla and Z3 rejected the counterexample `x5 != 3`.

For the false claim `x5 = 4`, Isla and Z3 found the counterexample `x5 = 3`.

A forced timeout returned an error status. These measurements establish route feasibility, not full machine coverage.

The matching Isla `7f6882b` engine and `rv64d.ir` snapshot produced ADDI semantics in 0.75 seconds.

The trace wrote `x5 = 3`. The instruction-footprint report named `x5` as the written register.

The public Hyperray operation traced legal and illegal encodings in 1.31 seconds. It returned one distinct trace digest for each encoding.

The independent inventory validator accepted both real records. Its result reported complete instruction coverage for that two-instruction input.

The measured run also emitted unavailable-primitive diagnostics. The diagnostic gate recorded a disposition for each diagnostic.

The completed diagnostic gate recorded six dispositions for each real instruction trace. The stale configuration warning stopped a separate real run.

The public loader then produced four instructions from a real RV64 ELF. Isla traced all four, and the independent validator reported complete coverage.

These results close the instruction-to-Sail coverage leaf. They do not close Sail-to-circuit or environment coverage.

## Same-program trusted route

The first complete machine slice can trust one pinned Isla release instead of translating each Sail event into a Hyperray circuit.

```text
one bounded Isla program
  -> Isla semantic dump
  -> Isla solver query
  -> exact identity checks
  -> Hyperray verdict
```

Both Isla tools receive the same program, Sail model, and configuration. Hyperray creates both operations from one request.

Hyperray accepts a verdict only when these values match:

- The Isla release version.
- The program digest.
- The Sail model digest.
- The Isla configuration digest.

The semantic dump must contain every thread tree, executed instruction encoding, final assertion, memory section, and opcode footprint.

The executed instruction set and opcode-footprint set must be equal in both directions. A malformed tree, unknown diagnostic, or set mismatch stops verification.

`PROVED` means the pinned Isla solver found no counterexample for this exact bounded program. `DISPROVED` includes the solver state.

This route does not claim that Hyperray independently proved Isla. The pinned Isla tools, Sail model, configuration, and memory model remain in the trust base.

This route also does not close source-language provenance, external contracts, or the later Sail-to-circuit proof.
