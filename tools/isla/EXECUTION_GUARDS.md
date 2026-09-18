# Execution-guard source artifact

`patches/execution-guards-v1.patch` applies directly to upstream commit
`7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
It includes the earlier source changes. It must not apply after the older patches.
All 73 exported files matched the patched clean checkout.
The local build uses a separate target directory and preserves the older measured binaries.

Both whole-program tools use the existing native instruction-visit guard.
The trace tool retains error mode. It does not discard paths at the limit.
`docs/isla-execution-guards.md` defines the SDK argument and evidence contract.

The native workspace passed 158 tests.
All 24 listed real-tool cases have passing results across the recorded commands.
The initial full command failed one obsolete error-text assertion.
The corrected assertion passed separately. The additional repair test also passed.
`measured-execution-guards.json` retains these separate results and the exact tool identities.

This artifact does not establish full coverage or the total-process memory target.
