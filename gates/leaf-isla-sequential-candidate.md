# Gates: Sequential candidate dispatch

Scope: Reuse the candidate solver under the declared sequential memory contract.

- [x] G1: Direct candidate emission preserves trace SMT and the final assertion.
  EVIDENCE: run_litmus.rs retains write_events_with_opts with WriteOpts::smtlib before the profile dispatch. The sequential branch calls checked sequential_assertion. The existing solver process and result callback remain outside the dispatch. The axiomatic branch retains smt_of_candidate and CAT emission.
- [x] G2: Native mixed-width candidate assertions permit the result and reject its mutation.
  EVIDENCE: cargo test -p isla-axiomatic sequential_candidate passed 4 tests with 0 failures and 0 ignored in 0.05s. The actual assertion helper permits 0x2211 from two byte writes and rejects its negation. The existing Model and final_value reader returns the concrete value. A constant-only query also passes without CAT symbols.
- [x] G3: Unsupported graph, extra SMT, and memory assertions return errors.
  EVIDENCE: Native tests rejects_graphs_and_extra_smt and rejects_memory_and_unresolved_register_assertions passed. Missing registers, unsupported register values, unresolved labels, and unresolved keyword arguments also return errors. The default axiomatic validation path accepts its existing requests.
- [x] G4: Native build and handwritten source limits pass.
  EVIDENCE: cargo build --release --bin isla-axiomatic --bin isla-litmus-dump --bin isla-footprint completed in 1m 56s. Existing upstream warnings remain. New modules pass rustfmt --check, with maximum file length 59 lines and maximum function length 28 lines. git diff --check passes for the three modified upstream files.
