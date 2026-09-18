# Gates: Rust compiler route

Scope: Inventory and lower every in-scope compiled Rust operation without source-pattern rules.

- [ ] G1: Every compiler-inventoried function instance has a proof root.
  EVIDENCE: pending

- [ ] G2: No MIR or machine operation is represented by `Other`, skipped, or silently omitted.
  EVIDENCE: pending

- [ ] G3: Standard Rust fixtures produce complete provenance and machine coverage.
  EVIDENCE: pending

- [x] G4: The Rust-only compiler driver stays inside the Rust adapter tree.
  CHECK: test -f adapters/rust/tools/mir-dump/Cargo.toml && test ! -e tools/mir-dump
  EXPECT: exit 0
  EVIDENCE: The source-layout move preserved the complete driver directory.
