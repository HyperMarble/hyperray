# Gates: Cargo native preparation

Scope: Cargo library packages through the existing native scalar checker.

- [x] G1: Cargo request and artifact decisions pass their tests.
  CHECK: cd adapters/rust && cargo test --lib && cargo test --test prepare && echo CARGO_REQUEST_OK
  EXPECT: CARGO_REQUEST_OK
  EVIDENCE: Finished `test` profile [unoptimized + debuginfo] target(s) in 0.01s | Running tests/prepare.rs (target/debug/deps/prepare-78833d33a85b9056)

- [x] G2: Real Cargo packages, feature changes, rejection cases, and replay pass.
  CHECK: go test -count=1 -tags preparation_integration -run TestCargoPreparation -v ./execution && echo CARGO_MATRIX_OK
  EXPECT: CARGO_MATRIX_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution	8.127s | CARGO_MATRIX_OK

- [x] G3: The complete earlier native preparation matrix still passes.
  CHECK: go test -count=1 -tags preparation_integration -run TestPreparation -v ./execution && echo CARGO_NATIVE_REGRESSION_OK
  EXPECT: CARGO_NATIVE_REGRESSION_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution	19.745s | CARGO_NATIVE_REGRESSION_OK

- [x] G4: Rust and Go quality gates pass.
  CHECK: (cd adapters/rust && cargo fmt --all -- --check && cargo test --all-targets && cargo clippy --all-targets -- -D warnings -W clippy::too_many_lines -W clippy::excessive_nesting -W clippy::unwrap_used -W clippy::expect_used -W clippy::panic) && go test ./... && go vet -tags preparation_integration ./... && echo CARGO_QUALITY_OK
  EXPECT: CARGO_QUALITY_OK
  EVIDENCE: Checking hyperray-rust v0.1.0 (/Volumes/Hak_SSD/hyperray/adapters/rust) | Finished `dev` profile [unoptimized + debuginfo] target(s) in 3.13s

- [x] G5: Source limits, public construction, input preservation, and limitations pass review.
  EVIDENCE: The source audit covered 32 files: maximum file 65 lines, maximum function 35 lines. Public Cargo construction and JSON round-trip passed in tests/prepare/cargo.rs. TestCargoPreparationMatrix compared the entire fixture inventory and SHA-256 file hashes before and after preparation. Four Cargo cases and seventeen native cases passed in /tmp/hyperray-cargo.ZuzT99/matrix.log. docs/cargo-preparation.md states the scalar, native, offline-resolution, and runtime-only memory boundaries.
