# ARM64 macOS connection plan

Status: active design, 2026-09-08. This is not a support declaration.

## Requested outcome

Preserve RISC-V. Add native ARM64 macOS code to the verification path:
Rust compiler -> linked Mach-O -> exact loaded bytes -> Arm Sail model in
Isla -> SMT. Do not substitute a Rust Linux ELF for the Mach-O input.
Coverage means that the selected target's program operations reach the
solver. A proof result applies to a declared finite task and its boundary.

The user requires one complete logical change per commit. Each change
includes relevant tests and independent review before its commit and push.
Do not combine header validation, memory loading, and execution changes.
Optimization and broad cleanup are separate later work.

## Existing source evidence

- `machine/load.go` loads only the RISC-V ELF profile.
- `machine/image.go` supplies byte, permission, region, and instruction records.
- `machine/isla/program_render.go` currently writes `arch = "RISCV"`.
- `machine/isla/footprint_arguments.go` currently initializes `PC`, not `_PC`.
- Local `hyperray-research/isla-snapshots/armv8p5.ir` supplies an Arm model.
- The Arm config in `hyperray-research/isla-7f6882b/configs/armv8p5.toml`
  names `_PC`. Its toolchain uses an ELF carrier for litmus data sections.
- Native `isla-axiomatic/src/sequential_setup.rs` in the isolated FP tree
  admits only one RISCV thread. The memory callback needs Arm acceptance,
  not merely removal of this admission check.
- Arm startup calls reset before instruction execution. Its initialization
  overwrites SP and control state. Existing native thread `reset` values
  apply after model initialization. Go must expose that distinction.
- The Arm main loop exits on a zero instruction before announcement.
  That exit is not evidence of a requested function return.

## Logical implementation stages

1. **Header policy.** Add concrete `machine/arm64.ValidateHeader` with public
   errors. Use the standard `debug/macho` parser in callers. Test actual
   compiler-produced Mach-O headers and explicit rejection precedence.
   Header success does not establish load-command or runtime support.
   This API accepts parsed header fields. It does not promise safe parsing
   of hostile bytes through `debug/macho`, which is not hardened.
2. **Bounded format inspection.** Validate selected-slice and load-command
   ranges before parser allocation. Preserve format identity and commands.
   Thin ARM64 is the first format. Fat selection and arm64e remain explicit
   requirements, not silent host-dependent fallback.
3. **Memory mapping.** Map exact file bytes and zero-fill at declared virtual
   addresses. Preserve permissions and validate ranges and overlaps.
   Retain `__PAGEZERO` as an inaccessible reservation, not allocated bytes.
   Keep fixups, binds, dependencies, TLS, and initialization obligations.
   An unsupported obligation returns a named blocked result or rejection.
   It must not become successful loading with an ignored effect.
4. **Code inventory.** Frame four-byte Arm encodings only in validated code
   sections. Executable segments also contain headers and data. Do not call
   all their words instructions. Mixed code/data needs explicit handling.
5. **Program identity and rendering.** Build the existing opaque Isla Program
   from Mach-O bytes with an explicit Arm profile. Render `AArch64`, use the
   correct footprint PC, and prevent pairing an Arm program with an RV
   model/config. Preserve RISC-V constructors and results.
6. **State and termination.** Expose post-reset registers, declared stack
   memory, and a caller return boundary. Preserve LR and SP under the model's
   startup sequence. Distinguish return from model exit, trap, timeout, and
   PC-visit exhaustion. Do not inject a fake zero instruction as a return.
7. **Arm memory semantics.** Inspect Arm memory-kind and store-result use.
   Connect only validated behavior to the sequential backend. Test actual
   Arm load/store forwarding and failures before changing admission.
8. **Native acceptance and remaining runtime work.** Exercise a real linked
   Mach-O through the public API and SMT. Retain separate requirements for
   dynamic linking, libSystem, syscalls, TLS, allocation, signals, and threads.
   A closed leaf-function example does not complete these requirements.

## Acceptance evidence

For each stage, record its exact input files, command, exit status, and
reviewed source identity. The initial execution cases must include:

- A compiler-produced arithmetic result and a wrong-result counterexample.
- An observation of caller-supplied SP after model reset.
- Actual Arm memory writes followed by reads.
- Branches and calls with a declared successful return reason.
- Rejection or distinct results for unsupported dependencies and bad exits.
- Existing RISC-V regression tests with unchanged results.

Bind the Mach-O digest, selected slice, model, config, and tool identities.
An internal ELF data carrier is transport only. Its bytes must match the
original Mach-O mapping. It cannot replace the original artifact identity.

## Current limits

No ARM64 execution acceptance exists yet. The native memory and return
contracts need implementation. The local Arm model is not evidence that all
Apple extensions work. A dependency-free static analysis fixture can test
initial connections, but cannot establish ordinary macOS process support.
