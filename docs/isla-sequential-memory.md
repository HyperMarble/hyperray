# Early memory constraints

The full finite-program objective remains unchanged. This integration repairs
memory-dependent path exploration. It does not establish full coverage.

The pinned Isla executor creates fresh values for symbolic memory reads.
The axiomatic memory constraints enter after trace collection. A fresh pointer
can thus select a fault path before the later constraints reject that path.
The stored-pointer fixture encountered `NoFunction("trap_callback")`.
The integrated experiment must establish the cause of this particular error.

## Reused mechanism

The source is `isla-testgen`, commit
`cc4c613410766006c8a14ecf8787c66839c17597`, `src/execution.rs`.
Its sequential memory callback uses SMT byte arrays. Stores update bytes.
Reads assemble bytes in little-endian order. Branches receive separate memory
states through the existing callback clone operation.

Hyperray must not copy the address restrictions or tag assumptions from that
test generator. Invalid addresses must remain errors or modeled fault states.

## Explicit contract

The new optional memory profile is `sequential`.
The default remains the existing axiomatic profile.
The public caller selects the profile. The generated artifact records it.
Both trace extraction and candidate evaluation use that artifact.

Both sequential program stages enable the existing Sail `__monomorphize_reads` control in their initial state.
This control partitions symbolic memory addresses without discarding alternatives.
Both stages keep the original configuration for their separate footprint analysis.
The independent footprint request also keeps its original arguments.
`docs/isla-symbolic-pc.md` records the source and the function-table experiment.

The sequential profile requires one modeled thread, ordinary RAM, immutable
code, little-endian 64-bit addresses, and no external memory writer.
Interrupts, page tables, custom regions, exclusive accesses, and memory tags
require their own semantics. The first integration returns explicit errors
for these operations. It must not discard their paths or assert them away.

The initial array contains the loaded bytes. Other bytes remain symbolic.
The sequential profile uses this explicit initial-state contract.
Ordinary writes update memory after access validation.
Their primitive Boolean does not represent an ordinary store fault.
The callback leaves that Boolean unconstrained.
`docs/isla-ordinary-write-contract.md` records the source and compiled evidence.
The callback must return conversion and unsupported-operation errors.

The array mechanism supports byte overlap. The measured axiomatic encoder
cannot assemble a read from several differently sized writes. Its whole-read
constraints reject a valid byte-array execution. Thus, the sequential profile
uses the existing trace SMT and final assertion without the axiomatic encoder
or CAT constraints. The default axiomatic route remains unchanged.

The sequential contract is not an equivalence claim for an arbitrary CAT model.
Its artifact digest binds the selected memory contract. Extra memory assertions,
read-from enumeration, and graph requests require separate support or explicit
errors. They must not silently enter the sequential route.

## Required evidence

Native solver tests must cover stored symbolic pointers, byte overlap, initial
bytes, rejected writes, branch copies, and explicit unsupported-operation errors.
Both Boolean values must preserve the ordinary store's bytes.
Each equality proof also requires a satisfiable normal execution.

The public end-to-end test must retain the compiler-built stored-pointer case.
It must prove the declared result and reject a changed result.
A real invalid-pointer case must remain an error or a counterexample.
The invalid-stack regression requires no verdict and an explicit trace-setup error.
It requires the missing `trap_callback` and the `zhandle_mem_exception` path.
Source-location details must not cause rejection of this same error.
Compiler-built call, recursion, dispatch, and symbolic-input tests remain.

No claim of full coverage follows from these tests. The full architecture still
requires coverage equality, complete finite exploration, and model equivalence.
