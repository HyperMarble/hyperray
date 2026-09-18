# Sail event-interface audit

Status: investigation. This route does not authorize a proof verdict.

## Purpose

The imported sequential model selects fixed undefined values and omits barrier effects.
The upstream `Sail.ArchSem` interface retains instruction effects and finite choices.
This audit establishes which parts Hyperray can reuse without instruction-specific rules.
The coverage contract in `PLAN.md` remains unchanged.

## Acceptance

The generation experiment uses the pinned RISC-V source without edits.
It selects `CONCURRENCY_INTERFACE_V2` through the official compiler option.
It retains compiler output, errors, source revisions, and the configuration.
Generated files stay outside the imported model directory.
A generation error is evidence of an integration gap, not proof of an unsupported language.

Lean proofs refer directly to the pinned `Sail.ArchSem` definitions.
They must retain all finite choices and the continuations after instruction effects.
Their statements quantify over values, bounds, and continuations instead of program fixtures.
Negative proof fixtures must fail for the claimed semantic difference, not an import error.
An upstream mismatch remains explicit. The audit must not silently replace its behavior.

## Completion boundary

This audit does not prove compiler correctness or the whole RISC-V model.
It does not supply a scheduler, a memory-order model, or external contracts.
A successful proof about an event interface does not establish executable coverage.
The machine branch remains open until its existing integration gates pass.

## Measured primitive evidence

Lean accepted nine theorems against Sail support revision
`079463134b9c50450b8393e1566a09fc492a34d9`, with Lean 4.29.0.
The theorems preserve choice continuations, instruction effects, barriers, and bitvector choices.
They also establish an upstream range mismatch.
The replay guards the exact axiom dependencies and rejects two false claims.
It does not accept an import error as a successful negative test.

The source contract is `sail/doc/asciidoc/language.adoc:106` at revision
`5745ea9e5369ab4fc51de6f8b773dd8ebc323357`.
It defines `range` as inclusive of both endpoints.
But `ArchSem.PreSail.undefined_range` requests only `(upper - lower).toNat` choices.
Every reply is less than the upper endpoint.
A singleton range has no reply.
The imported library remains unchanged, so this defect remains present.

## Integration decision

The event interface is not a direct replacement for the imported RISC-V model.
The RISC-V memory layer uses the older request types and instantiation parameters.
The newer interface rejects its `pa` parameter before Lean generation.
The repeat generation script exited successfully with records in
`/tmp/hyperray-event-generation.vDNAyP`.
Its older-interface control generated successfully. The audit did not compile that generated Lean model.
An integration must preserve the old request fields, memory results, and event order.
It must also resolve the range mismatch before an all-choice coverage claim.
Adding rules for individual Rust programs does not resolve these interface problems.

## Replay

```sh
bash proof/sail_events/shell/replay.sh MODEL_DIR LAKE_BIN
bash proof/sail_events/shell/generation.sh SAIL_SOURCE RISCV_SOURCE
```

The scripts require the pinned local sources and tools.
The generation script prints the directory for its output and tool digests.
Generation and model compilation are separate operations.
