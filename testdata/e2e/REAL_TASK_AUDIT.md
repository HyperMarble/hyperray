# Real-task E2E audit

Date: 2026-08-27

This ledger separates real task/PR evidence from reduced fixtures. A candidate is not positive E2E evidence unless its complete instruction-grounded concrete input universe, implementation scope, test predicate, runner, and frozen environment translate without approximation.

## Search scope

- Python: 41 Pluto bundles and 41 verifier modules; 15 Pluto `thanos_submit` bundles and 16 verifier modules; all 34 Python Deep-SWE tasks; 85 Semble commits; 268 OpenViking source-and-test commits of at most 200 changed Python lines; 1,909 MLflow source-and-test commits of at most 100 changed Python lines; targeted Shadow OSS histories.
- Rust/C++: all 14 top-level local Git repositories containing Rust or C++; all 113 Deep-SWE tasks; Pluto tasks and `thanos_submit`; focused histories including PyTorch, TensorFlow, Qdrant, jcode, ast-grep, Stim, and OpenViking.
- Rejected by construction: sampled subsets of unbounded integer/string/container domains, methods with unmodeled state, partial truth tables presented as complete, indirect or reduced verifier replacements, and source snapshots without trustworthy task/PR instruction provenance.

Strict positive tasks found: **0**.

## Faithful reconstructed task: jcode fallback picker

- Repository: `/Volumes/Hak_SSD/jcode`
- Commit: `f85c2d596f02d943dbb72e45a88e4e6071c9f8b7`
- Parent: `95087d57b3d5b5dd02c64a12c44e149d4426abad`
- Source: `src/tui/ui_inline_interactive.rs`
- Operation: `picker_row_marker(bool, bool, bool) -> &'static str`
- Concrete universe: all eight Boolean assignments.
- Test-blind inputs: `instruction.md`, `provenance/base-metadata.json`, and `patches/solution.patch`
- Verifier patch: `real-rust-jcode-picker-negative/patches/tests.patch`
- Byte-exact base/solution source snapshots: `real-rust-jcode-picker-negative/source/`
- Frozen upstream Cargo/CI environment metadata: `real-rust-jcode-picker-negative/environment/`

The two patches reconstruct the source commit tree exactly. The full source verifier is contradictory: an existing assertion requires `picker_row_marker(true, false, true) == "▸"`, while the added assertion and solution require `"⚠"`. The faithful handwritten `TestsPass` set is therefore empty.

The frozen v0.10 architecture requires independent translation of that real verifier. It cannot be replaced with a generated verifier, a copied/reduced function, or a hand-authored surrogate.

Current blockers are generic and unresolved: the operation is private inside a broad 39 KiB Rust module that cannot be compiled as a standalone `rustc --test` file; the full crate imports and nested tests exceed the current compiler-closed frontend subset; and the exact verifier contains conflicting assertions for one assignment. Until the production CLI returns the corresponding exact false-negative witness—or blocks explicitly on the unsupported full-module semantics—this candidate is not positive evidence.

## Python near-misses

### OpenViking memory policy

- Commit: `ad2468979f0e75d7f23345638841fc67db6b3b7f`
- Source: `benchmark/locomo/vikingbot/import_to_ov.py`
- Test: `tests/unit/test_locomo_peer_wiring.py`

`build_memory_policy(group_chat: bool)` is exercised for both Boolean values, but returns a nested dictionary/list, lives in an importful benchmark module, and the verifier assigns an expected structure before asserting. No local task instruction establishes the Boolean universe. This is a frontend and provenance blocker.

### MLflow health endpoint

- Commit: `8531fb3ef404067ad57907cada0a354213f804df`
- PR: `#2725`
- Source: `mlflow/server/__init__.py`
- Test: `tests/server/test_handlers.py`

The zero-argument `health` endpoint is decorated and import-time application state is semantically required. It returns an HTTP tuple, while the verifier uses a Flask context manager, assignments, attributes, and method calls. This is not a pure zero-input function task.

### Pluto/Deep-SWE candidates

The syntactically smallest Pluto Python tasks (`arange_step_dtype_guard`, `csv_quoted_newline_iterator`, `link_header_pagination_walker`, `password_rotation_history_guard`, and `semver_range_prerelease_guard`) all expose unbounded numeric, string, stream, list, object, or mutable-state inputs. The existing SemVer and Link E2E bundles are stress/blocker artifacts; their finite examples do not bound the task instructions.

## Rust near-miss

### OpenViking CLI picker eligibility

- Commit: `7ae22460deb134a27fb2f76d5e74ebb92cd1b152`
- PR: `#2198`
- Source/test: `crates/ov_cli/src/main.rs`

`language_command_can_run_picker(bool, bool) -> bool` is a pure free function, but its native verifier omits `(true, true)`, the commit is an 11,365-insertion CLI rewrite, and the function/test are embedded in an unsupported broad binary module. It is neither an exact positive nor a focused task.

The existing `real-rust-capture` task remains a faithful stateful-method blocker; no receiver-specific production adapter is allowed.

## C++ near-misses

### Stim Pauli encoding snapshot

- Repository snapshot: `/Volumes/Hak_SSD/tmp_stim_audit`
- Commit: `79ae4f118ca11c615d6d8de7c6eed7d189d3a6eb`
- Source: `src/stim/stabilizers/pauli_string.h`
- Test: `src/stim/stabilizers/pauli_string.test.cc`

`pauli_xz_to_xyz(bool, bool)` has all four direct GTest assertions and an exact production doc comment. The retained history does not identify a trustworthy focused task/PR, and the implementation requires unsupported `uint8_t`, C-style casts, XOR, OR, and shift semantics. It is a benchmark seed, not task proof.

### PyTorch PrivateUse1 case PR

- Commit: `343071cd9670301aab31e35d7e12ccd028da67ac`
- PR: `#132980`
- Source: `c10/core/DeviceType.cpp`
- Test: `c10/test/core/Device_test.cpp`

Both values of `lower_case` are asserted, but output also depends on mutable global backend registration. Exact semantics require atomics, a global string, iterators, `std::transform`, and a function pointer. The Boolean parameter alone is not a closed input universe.

### Existing task stress bundles

`real-cpp-ssd-fits` has a sound six-category test-blind Phase-A candidate, but the relational categories are unbounded and no compiler/model-checker universality proof is available. `real-cpp-continue` is a faithful receiver/state/side-effect blocker. Neither is positive E2E proof.
