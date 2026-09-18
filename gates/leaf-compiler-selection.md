# Gates: explicit compiler build selection

Scope: correct feature selection and fresh compiler inventories, not full semantic coverage.

- [x] G1: Public compiler requests and dump-read errors pass their decision tests.
  CHECK: cd adapters/rust && cargo test --test compiler_selection && cargo test --lib && echo COMPILER_SELECTION_DECISIONS_OK
  EXPECT: COMPILER_SELECTION_DECISIONS_OK
  EVIDENCE: Finished `test` profile [unoptimized + debuginfo] target(s) in 1.73s | Running unittests src/lib.rs (target/debug/deps/hyperray_rust-d42bc9d2e489c86c)

- [x] G2: Real compiler selection and earlier compiler integration tests pass explicitly.
  CHECK: cd adapters/rust && (cd tools/mir-dump && cargo build --release) && compiler_output=$(mktemp -d /tmp/hyperray-compiler-gate.XXXXXX) && HYPERRAY_MIR_DUMP=/Volumes/Hak_SSD/hyperray/adapters/rust/tools/mir-dump/target/release/mir-dump HYPERRAY_COMPILER_TEST_OUTPUT="$compiler_output" cargo test --test compiler_selection --test compiler_extraction -- --ignored --nocapture && echo COMPILER_SELECTION_LIVE_OK
  EXPECT: COMPILER_SELECTION_LIVE_OK
  EVIDENCE: Running tests/compiler_extraction.rs (target/debug/deps/compiler_extraction-001effee1ed406e1) | Running tests/compiler_selection.rs (target/debug/deps/compiler_selection-b0148fb713542846)

- [x] G3: Adapter and Cargo preparation regression checks pass.
  CHECK: (cd adapters/rust && cargo fmt --all -- --check && cargo test --all-targets && cargo clippy --all-targets -- -D warnings -W clippy::too_many_lines -W clippy::excessive_nesting -W clippy::unwrap_used -W clippy::expect_used -W clippy::panic) && go test -count=1 -tags preparation_integration -run TestCargoPreparation -v ./execution && echo COMPILER_SELECTION_REGRESSION_OK
  EXPECT: COMPILER_SELECTION_REGRESSION_OK
  EVIDENCE: Running tests/shape_clippy.rs (target/debug/deps/shape_clippy-7b9a4e1df8c6ea7a) | Finished `dev` profile [unoptimized + debuginfo] target(s) in 0.18s

- [x] G4: Public construction, source limits, fixture preservation, and coverage boundaries pass review.
  EVIDENCE: Twenty source files measured at most 70 lines per file and 30 lines per function. CompileRequest construction, serialization, and invalid-input rejection passed. The real compiler matrix compared all source fixture paths and bytes before and after eight selections. /tmp/hyperray-compiler-final.R7gc4m/compiler.log records ten fresh dumps and the missing-inventory rejection. docs/compiler-selection.md states that this selected host build is not every feature combination or full semantic coverage.
