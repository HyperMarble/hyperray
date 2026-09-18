# Gates: initialized memory backend

Scope: Generic loaded bytes and writable sections in the pinned Isla model.

- [x] G1: Public initialization preserves bytes and rejects invalid regions.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/isla-7f6882b && cargo test -p isla-lib memory::initialized_memory::tests
  EXPECT: test result: ok. 3 passed
  EVIDENCE: Native tests passed for adjacent sections, reservation splits, empty input, overlap, and address overflow.
- [x] G2: Writable reads remain symbolic memory events.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/isla-7f6882b && cargo test -p isla-lib memory::initialized_memory
  EXPECT: test result: ok. 10 passed
  EVIDENCE: All 10 tests passed. Reads retain symbolic events. Cross-section reads retain bytes. Concrete and possible symbolic RO stores return errors.
- [x] G3: Solver predicates preserve symbolic addresses and overlapping read widths.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/isla-7f6882b && cargo test -p isla-axiomatic smt_events::initial_memory
  EXPECT: test result: ok. 3 passed
  EVIDENCE: Three native tests passed. Widths 1, 2, 4, and 8 each returned SAT before the incorrect-value mutation returned UNSAT.
- [x] G4: Native regressions and source format pass.
  CHECK: cd /Volumes/Hak_SSD/hyperray-research/isla-7f6882b && cargo test -p isla-lib -p isla-axiomatic --lib && rustfmt --edition 2021 --check --config skip_children=true isla-lib/src/memory.rs isla-axiomatic/src/smt_events.rs isla-lib/src/initialized_memory/*.rs isla-axiomatic/src/initial_memory/*.rs isla-axiomatic/src/initial_memory/tests/*.rs && git diff --check
  EVIDENCE: Final command exited 0 after the validator refactor. All 21 isla-axiomatic and 75 isla-lib tests passed. Rustfmt and git diff whitespace checks passed. Existing dependency warnings remain.
- [x] G5: New modules pass the measured Clippy rules without new diagnostics.
  EVIDENCE: Clippy completed with exit 0. The JSON diagnostic filter found zero messages in the new modules with too_many_lines, excessive_nesting, unwrap_used, expect_used, and panic enabled. Inherited dependency diagnostics remain.
