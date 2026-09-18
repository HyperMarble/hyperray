# Reproducible Isla source

The measured tools contain more changes than the earlier candidate-output patch.
The executable-entry, loaded-memory, sequential-memory, and unique-read changes require a complete source artifact.
The earlier patch and measurement remain unchanged.

The new patch starts from upstream commit `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
It includes the current tracked source changes, new source files, tests, and the sequential-memory notice.
The export uses a temporary Git index. It does not stage files in the user's real index.
Unexpected paths stop the export instead of disappearing from the artifact.

A fresh local clone must accept the patch without an earlier Hyperray patch.
Every exported file must match the measured working source byte for byte.
The native workspace tests must pass in that clone.
The clone must build `isla-axiomatic`, `isla-litmus-dump`, and `isla-footprint`.
The rebuilt tools must pass a public memory-dependent program and its changed requirement.

The measurement records the patch digest, upstream commit, Rust and native solver versions,
native solver digest, input digests, and rebuilt executable digests.
The record is a local measurement, not a trusted distribution manifest.
Source reconstruction does not establish identical executable bytes across build paths or machines.
The artifact does not establish full semantic coverage.

## Measured result

The patch applied to a fresh clone. All 49 exported files matched the working source byte for byte.
The clean native workspace passed 140 tests, with no failed or ignored tests.
The clean release build produced all three tools in 4 minutes 14 seconds.
The public function-table test passed all three queries with those tools in 126.57 seconds.
Both compiler-retained target functions had entry events. The counterexamples contained the declared outputs `17` and `64`.

`tools/isla/EXECUTABLE_MEMORY.md` gives the build procedure.
`tools/isla/measured-executable-memory.json` records the measured digests.
All nine recorded file digests matched a fresh measurement.
The original Git index and both earlier release artifacts retained their original digests.
The logs remain in `/tmp/hyperray-isla-source.oWNCM0/`.
