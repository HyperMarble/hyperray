# Sail and Isla Integration

Status: ACCEPTED ROUTE. The proposal bridge works. Instruction coverage integration is in progress.

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

The Isla result does not establish semantic coverage by itself. Hyperray accepts it only after the coverage certificate accepts these sets:

```text
program instructions
= decoded Sail cases
= executed Isla semantic regions
= solver regions
```

The comparison works in both directions. A missing, extra, duplicate, or stale member causes an engine error.

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

The instruction inventory and trace inventory must contain the same addresses and bytes. Missing, extra, changed, or duplicate records cause an engine error.

An unmatched RISC-V encoding maps to Sail's declared illegal-instruction case. It is a modeled trap, not an omitted instruction.

This route establishes instruction-to-Sail coverage. It does not establish Sail-to-circuit equality or complete environment behavior.

## Program independence

Production code must not contain fixture names, function names, instruction bytes, addresses, or property values.

Tests use different programs with the same public operation. A program change changes input artifacts, not Hyperray source.

## Measured proof slice

The local proof built the official Sail-to-Isla plug-in and the Isla engine. Sail generated a 15,663,211-byte RISC-V IR file.

For `addi x5,x0,3`, Isla and Z3 rejected the counterexample `x5 != 3`.

For the false claim `x5 = 4`, Isla and Z3 found the counterexample `x5 = 3`.

A forced timeout returned an error status. These measurements establish route feasibility, not full machine coverage.

The matching Isla `7f6882b` engine and `rv64d.ir` snapshot produced ADDI semantics in 0.75 seconds.

The trace wrote `x5 = 3`. The instruction-footprint report named `x5` as the written register.

The public Hyperray operation traced legal and illegal encodings in 1.31 seconds. It returned one distinct trace digest for each encoding.

The independent inventory validator accepted both real records. Its result reported complete instruction coverage for that two-instruction input.

The measured run also emitted unavailable-primitive diagnostics. Coverage remains incomplete until each diagnostic has a proved disposition.
