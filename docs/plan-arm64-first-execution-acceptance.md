# First ARM64 public execution acceptance

Status: approved test-first contract, 2026-09-09. Production APIs are proposed.

## Goal

Connect the real compiler-derived Mach-O fixture to the existing Isla verifier.
This first case is a closed function. It does not establish Darwin process or
runtime support. RISC-V behavior remains unchanged. No push precedes the
requested ARM64 end-to-end acceptance.

## Measured input

fixtures/machine/arm64/tiny-arm64-static contains 16,456 bytes.
SHA-256: a55e2554b91b1a5d451df1b1125a883f9f239f610ba841caed55a75651574ac4.
The __text interval is [0x1000002e8, 0x1000002f0), at file offset 744.
Its bytes are 00 04 00 91 c0 03 5f d6. The sole text symbol is
_arm64_fixture. The retained Rust source returns value.wrapping_add(1).
These observations establish this fixture boundary only. Production must not
infer arbitrary function extents from neighboring symbols or section size.

## Proposed public input

BuildARM64Program(content []byte, maximumLoadedBytes uint64,
boundary ARM64ProgramBoundary) (Program, error).

ARM64ProgramBoundary fields:
- Name string
- FunctionStart uint64
- FunctionEnd uint64
- ReturnAddress uint64
- PostResetRegisters []RegisterValue
- NegatedAssertion string
- MaximumProgramBytes uint64.

The half-open function boundary belongs to the original image. The declared
continuation is aligned and outside loaded executable ranges. R30 contains
the continuation. PostResetRegisters explicitly means after Sail reset.
The first leaf uses no added stack bytes. SP_EL0 remains an explicit input.
A later memory-using case needs its own bounded memory contract.

## Proposed public output

VerificationResult gains TerminalEvidence []TerminalEvidence.
TerminalEvidence contains CandidateIndex uint64, ThreadIndex uint64,
Kind TerminalKind, and DeclaredAddress uint64.
TerminalKind is a named string type. BoundaryReached has value boundary_reached.
CandidateIndex is the zero-based States-row ordinal. ThreadIndex is the
modeled thread ordinal, not the executor TaskId. Every accepted candidate/thread
pair has exactly one record with the expected boundary reason and address.

Native records must come from the actual terminal path, not copied query
metadata. The retained terminal register state remains authoritative for PC.
No concrete observed register values are invented for UNSAT candidates.
The existing CounterexampleState supplies actual SAT witness values.
No second register-value representation is necessary.

Proven-inconsistent paths do not become solver candidates. Dead is not an
accepted terminal record. No feasible candidates means no successful verdict.
Unexpected exit, natural completion, timeout, unknown solver result, and
PC exhaustion cannot become BoundaryReached. Earlier errors retain precedence.

## Required test

Dedicated external isla_test files use tags isla_integration && arm64_acceptance.
When these tags are enabled, missing prerequisites fail rather than skip.
Use the actual fixture bytes, the Arm model, and measured native tool identities.
A RISC-V release cannot satisfy this acceptance through a renamed architecture.

Set post-reset R0=3, R30=continuation, and a declared SP_EL0 value.
The correct negated query expresses a violation of result R0=4, unchanged SP,
or PC=continuation. Require Proved and complete candidate/thread terminal rows.
The wrong-result query claims R0=5. Require Disproved and decode actual R0=4,
SP_EL0, and _PC from the real CounterexampleState. Reject substring-only matches.
Both cases retain the exact original image digest through Program and result.
No fake sentinel bytes, replacement instruction semantics, mock solver, or
Linux source rebuild can satisfy the test. First record a genuine red result
for absent APIs. Do not add production stubs just to compile the test.
