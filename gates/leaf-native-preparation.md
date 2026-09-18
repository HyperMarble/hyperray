# Gates: automatic native checker preparation

Scope: Reusable Rust preparation for the declared native scalar interface, not universal coverage.

- [x] G1: Public request decisions preserve bounds and reject malformed inputs.
  CHECK: cd adapters/rust && cargo test --test prepare && echo PREPARATION_API_OK
  EXPECT: PREPARATION_API_OK
  EVIDENCE: Finished `test` profile [unoptimized + debuginfo] target(s) in 1.53s | Running tests/prepare.rs (target/debug/deps/prepare-78833d33a85b9056)

- [x] G2: Every declared source-to-checker case passes through preparation and observation.
  CHECK: go test -count=1 -tags preparation_integration -run TestPreparation -v ./execution && echo PREPARATION_MATRIX_OK
  EXPECT: PREPARATION_MATRIX_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution	21.522s | PREPARATION_MATRIX_OK

- [x] G3: Rust format, adapter tests, and strict Clippy pass.
  CHECK: cd adapters/rust && cargo fmt --all -- --check && cargo test --all-targets && cargo clippy --all-targets -- -D warnings -W clippy::too_many_lines -W clippy::excessive_nesting -W clippy::unwrap_used -W clippy::expect_used -W clippy::panic && echo PREPARATION_RUST_OK
  EXPECT: PREPARATION_RUST_OK
  EVIDENCE: Checking hyperray-rust v0.1.0 (/Volumes/Hak_SSD/hyperray/adapters/rust) | Finished `dev` profile [unoptimized + debuginfo] target(s) in 3.00s

- [x] G4: Go regression tests and static analysis pass.
  CHECK: go test ./... && go vet ./... && echo PREPARATION_GO_OK
  EXPECT: PREPARATION_GO_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/tools/sail-jib/go	(cached) | PREPARATION_GO_OK

- [x] G5: Public construction, source limits, generated-source quality, and non-claims pass review.
  EVIDENCE: External Rust tests construct Request. The Go integration calls the JSON preparer, decodes execution.Request, and observes all 17 declared cases plus replay. The source audit measured 27 files, with maxima of 72 file lines and 32 function lines. Generated Rust and C format checks passed. Clang analysis returned exit 0 without user-code warnings. docs/native-preparation.md lists the exact native interface and remaining limits.
