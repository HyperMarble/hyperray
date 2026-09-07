# Rust Floating Capability Implementation Plan

> **For agentic workers:** Execute this plan inline. Do not spawn workers for this task.

**Goal:** Record and enforce the real boundary for Rust compiler-built floating-point ELFs when the pinned Sail model reaches an unavailable softfloat primitive.

**Architecture:** Add one minimal `f64` Rust fixture and one tagged integration test. The test uses the existing compiler, direct `ld.lld` path, ELF loader, public footprint operation, public semantic operation, and public solver operation. The test accepts only an explicit named `process_error` for the reachable primitive. Existing integer acceptance remains unchanged and proves that load-time diagnostics alone do not block integer programs.

**Tech Stack:** Rust nightly `2026-08-21`, Go integration tests, RISC-V RV64D ELF, pinned Isla release, Sail IR, Z3-backed Isla solver.

**Spec:** `docs/isla-integration.md` and the user task contract.

## Global Constraints

- Preserve the inherited dirty worktree.
- Change only the new fixture, its test, and supporting evidence if required.
- Use the local compiler, linker, Isla tools, and model inputs.
- Do not download files or delete unrelated files.
- Report load-time unavailable diagnostics separately from reachable unsupported operations.
- Require exact positive and negative observable outcomes. Do not convert a tool error into a proof.
- Keep cleanup/unwind, TLS, and multi-CGU as unmeasured until real evidence exists.

---

### Task 1: Add the smallest real floating fixture and red-first acceptance test

**Files:**
- Create: `fixtures/rust/machine/floating.rs`
- Create: `machine/isla/rust_floating_real_test.go`

**Interfaces:**
- Consume: Existing `compileRustExecutable`, `realExecutableVerifier`, `BuildProgram`, and real request helpers.
- Produce: A named real test that compiles and links an ELF containing `fadd.d`, then records the exact unsupported-operation error from public verification.

- [ ] **Step 1: Add `floating.rs`**

Use one `f64` addition. Do not add a source-level replacement rule.

- [ ] **Step 2: Add the tagged test**

Compile `floating.rs` with `compileRustExecutable`. Build a boundary with the ELF entry and return symbol. Enable the RV64D machine state with typed `misa` and `mstatus` values. Call the public executable verifier. Assert that the result is empty and the error has code `ProcessFail`, with detail containing the reachable `NoFunction("extern_f64Add"...)` error and the load-time `No primop softfloat_f64add` diagnostic.

- [ ] **Step 3: Run the test before any production change**

Run the exact integration test with all ten local environment variables set. Expected result: the test fails if the current public route reports success or hides the reachable unsupported operation. If the route already returns the exact error, retain the test as a passing capability guard and record that no production fix is needed.

- [ ] **Step 4: Implement only a root-cause fix if the red test identifies one**

Change the smallest public boundary that loses the reachable process error. Do not add a softfloat implementation or suppress the error. Add a focused unit test for the changed decision if production code changes.

- [ ] **Step 5: Run the red-first test again**

Expected result: the test passes with the exact reachable unsupported operation. The existing integer acceptance test must still pass with `PROVED` and `DISPROVED` results.

- [ ] **Step 6: Review and commit only the scoped change**

Obtain independent review with the original contract, changed paths, test output, and inherited-worktree warning. Commit only the fixture and test, plus a production fix if the evidence requires it.

---

### Task 2: Run named regressions and close the evidence record

**Files:**
- Modify: `docs/isla-integration.md` only if the measured evidence adds a stable capability fact.
- Create: `/Users/hak/hyperray-rust-real-floating-20260907.log` outside the repository.

- [ ] **Step 1: Inspect the broad log termination**

Use the existing tail and process evidence. Do not call the full suite again without a changed input or unresolved concern.

- [ ] **Step 2: Run remaining named cases**

Run only named cases not covered by the completed same-build acceptance, with the complete environment and internal cache/temp paths. Record each exit and log path.

- [ ] **Step 3: Run format, vet, and focused tests**

Run the changed Go tests, `gofmt`, and `go vet` for the affected package. Run the exact real integration test and the integer acceptance test.

- [ ] **Step 4: Update the capability table**

State which compiler, ELF, Sail, SMT, and unsupported-operation boundaries have direct evidence. Keep unsupported candidate areas named and unmeasured when no real test exists.
