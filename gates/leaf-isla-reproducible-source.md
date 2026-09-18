# Gates: reproducible Isla source

Scope: Rebuild the measured executable and memory integration from pinned upstream source.

- [x] G1: The patch includes every current Isla source change and applies to the pinned commit.
  EVIDENCE: executable-memory-v1.patch contains all 49 paths from the tracked-change and untracked-source inventories. It passed git apply --check and applied to a fresh local clone at 7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6. The patch SHA-256 is 5d13779ec58c8944e4e6502c03c776c8ee8bbf7edc07478af55a6071e4715883.

- [x] G2: The patched clean checkout matches every exported source file and passes its native tests.
  EVIDENCE: All 49 byte comparisons passed with SOURCE_FILES_IDENTICAL. cargo test --locked --offline --workspace exited zero in the fresh clone. The log contains 140 passing tests, zero failures, and zero ignored tests across its unit and documentation suites. /tmp/hyperray-isla-source.oWNCM0/native.log retains the result.

- [x] G3: The clean checkout builds the three tools and passes a public memory-dependent program with both query outcomes.
  EVIDENCE: The clean release build completed in 4 minutes 14 seconds. All three rebuilt tools passed TestRealRustSymbolicFunctionTable in 126.57 seconds. The output-set query returned PROVED, both target entries remained observable, and the two outcome queries returned counterexamples 17 and 64. build.log and clean-integration.log in /tmp/hyperray-isla-source.oWNCM0/ retain the results.

- [x] G4: The build record names source, patch, native dependency, toolchain, and executable digests without claiming binary reproducibility.
  EVIDENCE: tools/isla/measured-executable-memory.json records the pinned source, patch, Cargo lockfile, native library, toolchain, three executables, and three model inputs. All nine recorded file digests matched fresh measurements. otool -L identifies the recorded native library. tools/isla/EXECUTABLE_MEMORY.md gives the clean build and public integration commands. The record explicitly excludes binary reproducibility and full coverage claims.

- [x] G5: The export preserves the user's real Git index and the earlier candidate-output artifact.
  EVIDENCE: shasum -c passed for the real worktree index, candidate-status-v1.patch, and measured-release.json after export. /tmp/hyperray-isla-source.oWNCM0/preserved-before.sha256 records the original digests. Export staging used only /tmp/hyperray-isla-source.oWNCM0/export.index. No commit or push occurred.
