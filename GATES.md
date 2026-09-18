# Gates: Rust Stage 3 MODEL

Scope: Complete the Rust proof-model inventory without source-pattern bounds.

The loop-limit rule removal has a separate measured ledger:
`gates/leaf-remove-rust-loop-rules.md`. It does not complete Stage 3.

- [x] G1: The compiler diagnostics retain input shapes, cycles, and descendants without bound proposals.
  CHECK: cd adapters/rust && cargo test --test bound
  EXPECT: test result: ok.
  EVIDENCE: Six bound tests passed on 2026-09-05, including removal of serialized proposals.

- [x] G2: The public Stage 3 API records every loop from a CBMC GOTO inventory.
  CHECK: cd adapters/rust && cargo test --test model_inventory
  EXPECT: test result: ok.
  EVIDENCE: Finished `test` profile [unoptimized + debuginfo] target(s) in 0.01s | Running tests/model_inventory.rs (target/debug/deps/model_inventory-3d492c840ee42d85)

- [ ] G3: The current compiler driver operates on all configured Rust fixtures.
  CHECK: cd adapters/rust && cargo test --test extract_mir_run -- --nocapture
  EXPECT: test result: ok.
  EVIDENCE: Finished `test` profile [unoptimized + debuginfo] target(s) in 2.62s | Running tests/extract_mir_run.rs (target/debug/deps/extract_mir_run-bb77cd7f2faf7270)
  Its successful exit does not establish this gate. Two new compiler integration
  tests passed explicitly, but they do not cover all external Rust fixtures.

- [x] G4: All Rust adapter tests pass.
  CHECK: cd adapters/rust && cargo test --all-targets
  EXPECT: test result: ok.
  EVIDENCE: Running tests/prove_kani.rs (target/debug/deps/prove_kani-8f3e7084de27752c) | Running tests/shape_clippy.rs (target/debug/deps/shape_clippy-6868a9a5e224d684)

- [x] G5: Rustfmt accepts all Rust adapter files.
  CHECK: cd adapters/rust && cargo fmt --all -- --check
  EXPECT: exit 0 with no output
  EVIDENCE: `cargo fmt --all -- --check` returned exit 0 with no output on 2026-09-05.

- [x] G6: Clippy reports no warning under the workspace code rules.
  CHECK: cd adapters/rust && cargo clippy --all-targets -- -D warnings -W clippy::too_many_lines -W clippy::excessive_nesting -W clippy::unwrap_used -W clippy::expect_used -W clippy::panic
  EXPECT: Finished
  EVIDENCE: Finished `dev` profile [unoptimized + debuginfo] target(s) in 0.11s

- [ ] G7: Stage 3 has no hidden failure or policy-limit violation.
  EVIDENCE: pending

- [ ] G8: The Stage 3 design status and measured results match the implementation.
  EVIDENCE: pending
