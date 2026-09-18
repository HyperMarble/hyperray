# Gates: early sequential memory constraints

Scope: Reuse upstream byte arrays for explicit sequential memory during trace extraction.

- [x] G1: Native solver tests establish byte-array identities without vacuous proofs.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/isla-7f6882b && cargo test --locked --offline -p isla-lib sequential_memory::tests::pointer && echo POINTER_IDENTITIES_PASSED
  EXPECT: POINTER_IDENTITIES_PASSED
  EVIDENCE: The fresh pointer subset passed all three tests. The shared equality check requires SAT before UNSAT for the negated equality. Symbolic addresses, symbolic pointer values, and wide values remain in the tests. Two existing upstream compiler warnings remain outside this gate.

- [x] G2: Native public memory tests preserve rejected writes, branch copies, and explicit errors.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/isla-7f6882b && cargo test --locked --offline -p isla-lib sequential_memory && echo SEQUENTIAL_MEMORY_PASSED
  EXPECT: SEQUENTIAL_MEMORY_PASSED
  EVIDENCE: The fresh library suite passed 99 unit tests, including all seventeen sequential-memory tests, with zero failures or ignored tests. Both ordinary-store Boolean values preserve the written bytes. Actual errors preserve the prior bytes. Branch copies, overlap, unspecified bytes, explicit unsupported-operation errors, unique read values, and symbolic trace replay passed. native-final.log in /tmp/hyperray-unique-read.uazx9M/ retains the output. Upstream compiler warnings remain outside this gate.

- [ ] G3: The public API records the profile and rejects incompatible execution contracts.
  EVIDENCE: Partial. TestProgramMemoryProfileIsExplicit, TestExecutableRejectsMissingSequentialCapability, and TestSequentialMemoryCapabilityArguments passed. Four native sequential_candidate tests passed, including rejected extra contracts and model-value decoding. The complete execution-contract rejection matrix is not yet established.

- [x] G4: The unchanged stored-pointer program passes both positive and negative proof queries.
  EVIDENCE: TestRealRustPatterns/stack_array passed in 95.75 seconds after a fresh release build of all three Isla tools. The unchanged Rust fixture produced 30 static instructions, 30 observed instructions, and zero unobserved instructions. Its declared-result query returned PROVED. The changed-result query returned DISPROVED with 0:x10=#x000000000000000d;. Rust 1.98.0 and LLD 22.1.8 compiled the fixture. This is one fixture result, not full coverage or the 100 MB target.

- [x] G5: An invalid pointer remains an observable error or counterexample.
  EVIDENCE: TestRealRustSequentialInvalidStack passed again in 45.17 seconds with no verdict. The error contains trace setup, NoFunction("trap_callback", SourceLoc ...), and zhandle_mem_exception. The repeat log is /tmp/hyperray-unique-read.uazx9M/regressions.log. The unresolved trap callback remains an engine gap.

- [ ] G6: Mixed-width candidate semantics preserve the byte-array execution set.
  EVIDENCE: Partial. TestRealRustSequentialMixedWidth passed again in 86.97 seconds. The compiler retained two byte stores and one halfword load. All 20 instructions had execution events. The changed-result query returned 0:x10=#x0000000000002211;. The complete native workspace suite passed. The repeat log is /tmp/hyperray-unique-read.uazx9M/regressions.log. These regression results do not establish equality of the complete execution sets.

- [ ] G7: Parent replay, format, analysis, and reproducible tool patches pass.
  EVIDENCE: Partial. All fifteen listed real-tool tests passed in one command in 886.794 seconds, with no missing, failed, or skipped tests. All Go packages and integration-tagged go vet passed. The reproducible-source leaf passed all five gates: all 49 exported files matched a clean clone, 140 native tests passed, and all three clean rebuilt tools passed the function-table queries. The build retained upstream warnings. The logs are in /tmp/hyperray-multiple-targets.IMhJIb/ and /tmp/hyperray-isla-source.oWNCM0/.
