# Gates: Remove handwritten Rust loop-limit rules

Scope: Remove numeric-bound pattern inference and retain compiler extraction.
This change does not complete the reference model or its coverage proof.

- [x] G1: Remove the proposal generator, exclusive helpers, and public fields.
  EVIDENCE: Removed candidate, progress, start, step, stable, path, and relation.
  Removed Proposal, Direction, Evidence, Location, and the proposals field.
  Removed the unused trace::value and Expression::local entry points.
  The regression test rejects serialized reports with the old proposals field.
  Recovery archive: /private/tmp/hyperray-removed-loop-rules-20260905.tar.
- [x] G2: Preserve structural cycle reports and tool-generated loop inventory.
  CHECK: cd adapters/rust && cargo test --test bound --test model_inventory
  EXPECT: test result: ok
  EVIDENCE: Six bound tests and two inventory decoder tests passed on 2026-09-05.
  The existing Kani integration test skipped its external operation without configuration.
- [x] G3: Exercise the public extraction API with the real compiler.
  EVIDENCE: Built mir-dump with rustc 1.100.0-nightly (8925ea358 2026-08-20).
  Both compiler_extraction tests passed with --ignored --nocapture.
  The public extract::run API extracted counted, iterator, recursive, future,
  and generic bodies. The deliberate compile_error remained a compiler error.
  HYPERRAY_MIR_DUMP=/Volumes/Hak_SSD/hyperray/adapters/rust/tools/mir-dump/target/release/mir-dump
  HYPERRAY_COMPILER_TEST_OUTPUT=/private/tmp/hyperray-compiler-extraction.aJc1AC
- [x] G4: Pass adapter tests, formatting, and strict Clippy checks.
  EVIDENCE: cargo test --all-targets returned exit 0 on 2026-09-05.
  The live compiler tests require a separate explicit invocation, recorded in G3.
  Older tests retain configuration-dependent skips. This is not full fixture coverage.
  cargo fmt --all -- --check and strict Clippy returned exit 0.
  git diff --check returned exit 0. All changed source files fit 75 lines.
