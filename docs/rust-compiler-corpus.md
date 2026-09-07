# Rust Compiler Corpus and Floating Capability Evidence

Date: 2026-09-07

## Local compiler source

The selected compiler is the installed Rust toolchain:

- Toolchain: `nightly-2026-08-21-aarch64-apple-darwin`
- Compiler: `rustc 1.100.0-nightly (8925ea358 2026-08-20)`
- LLVM: `23.1.0`
- Source path: `/Users/hak/.rustup/toolchains/nightly-2026-08-21-aarch64-apple-darwin/lib/rustlib/rustc-src/rust`
- Source component: `rust-src (installed)`

The installed source component contains `compiler/` and `library/`. It does not contain the upstream `tests/` directory. It also does not contain a root `LICENSE` or `COPYRIGHT` file. The local compiler backend directories contain `LICENSE-APACHE` and `LICENSE-MIT` files. This change copies no compiler source or upstream test case.

## Existing project fixtures

The reusable machine fixtures are project-owned files under `fixtures/rust/machine/`. They contain no third-party license marker. Their SHA-256 identities are:

| Fixture | SHA-256 |
|---|---|
| `arithmetic.rs` | `20328ad10c09cc2850f2302f22d3eb4dfe52a968a212593ed611ace5ffea3c1b` |
| `atomic_shared.rs` | `c585c3158ae66321bdc1fb22a01d0ec8ca0c72426ad79a3a01962cf730c1ba50` |
| `branch.rs` | `dd1ed7024ecfe3d100c96d2aa96c40d8a7afc54bc218c412b3483afa4a1c3438` |
| `dynamic_call.rs` | `681fcc47eef346e41246acbc5c5e330434d469597f28f0c7deea4a62d8f1aa77` |
| `function_table.rs` | `a09ee34c1fadd809d3b581d08df00a6dd9c93e0f0d43e60d126719e5b8bbb099` |
| `generic_calls.rs` | `27e61463d8f4194d95f1889ce382d9ded00ac1da79e9ba560007462db30da0bb` |
| `initialized_memory.rs` | `6cb6d62977a055ec2f556b25e0f3cdecce29fd8586993c3abaead63b01d7f6e0` |
| `mixed_width.rs` | `a58dd30033c8b2610cbfc075b7b5628ba9db03a1dce06e4c655a57f7ac24da7d` |
| `recursive_calls.rs` | `79eb84ea2f520e0af40a951c9a8f39168c8596bea1e0372ab0df254d8cccd3ba` |
| `stack_array_bug.rs` | `501080ccf6d23f9333ab939b573d6744ad05e468f72c49ea8b78de4f847b53d0` |
| `stack_array.rs` | `c3fbc3460eb493b350036a5948314f6c937a3667d7a6511a3097a63a511386d4` |
| `stack_values.rs` | `1ee63ef174554ac14cd632da73ec972b6db9e289ee2b8571d249fc2fd17be877` |
| `floating.rs` | `f5a2732b662876f1e3c9470f97cf66b6684287ca6091f1a6fd877830760cadda` |

The new `floating.rs` fixture has two `f64` arguments and one `f64` addition. It avoids constant materialization, so the first unsupported floating operation is the arithmetic operation.

## Tool and model identities

The local Isla source checkout is at commit `75fe7b7d981a13293a8a4c82c341ec85c6c24b2d`. The selected release tools are:

| Artifact | Path | SHA-256 |
|---|---|---|
| Isla solver | `/Users/hak/hyperray-isla-execution-guards-20260907-target/release/isla-axiomatic` | `084ca7325762b50c4a3e7cc41bb8b3ef06d5b3b11afa354439e24c99394f4102` |
| Isla semantic dump | `/Users/hak/hyperray-isla-execution-guards-20260907-target/release/isla-litmus-dump` | `f5aa964c0476887f5229c09bc98f362804ec0c43bf069b89e5262d92b0c62da9` |
| Isla footprint | `/Users/hak/hyperray-isla-execution-guards-20260907-target/release/isla-footprint` | `97eed5245e1d807d4986d8b11823eefdc8b183c74a03c517339e551c99` |
| Sail IR | `/Users/hak/hyperray-isla-inputs-20260907/riscv_model_rv64d.ir` | `f775491d50fd8cf3d8641545f3ac93bd7230899630be7534f59925fda4ba4113` |
| Isla configuration | `/Users/hak/hyperray-isla-inputs-20260907/riscv64.toml` | `3a367ed417e668e558e0516a91823fe765d04ca4c0c38ae53dd65da2b2b0126a` |
| Memory model | `/Users/hak/hyperray-isla-inputs-20260907/riscv.cat` | `34c8dea2e531f0e932f50d7638173f04f7df210122c143a761de94b0fb289995` |

## Measured floating boundary

The compiler emitted a valid statically linked RV64D ELF. The ELF contained `fadd.d fa0, fa0, fa1` at address `0x80100000`. The ELF SHA-256 was `7d8bb283e91424d7e05afbacd8f45f2b53801537e07a9439c7bd35a06e9d5acb`.

The footprint command exited `0` for encoding `02f57553`. Its diagnostics listed `No primop softfloat_f64add`. The public executable test then reached `NoFunction("extern_f64Add"...)` and returned `isla process_error`. The result contained no partial executable proof. The generated Sail IR maps `extern_f64Add` to the unavailable `softfloat_f64add` function.

This separates two facts:

1. The model lists many unavailable `softfloat_*` functions during load.
2. The real `fadd.d` path reaches the specific `softfloat_f64add` function through `extern_f64Add`.

The other listed `softfloat_*` functions remain unmeasured by this fixture. The integer fixture does not reach any floating operation.

The existing integer acceptance path still returns `PROVED` for the correct property and `DISPROVED` for the changed property. Therefore, the load-time diagnostic list does not block integer programs by itself.

## Unmeasured candidates

The current corpus has no real ELF acceptance case for cleanup or unwind, thread-local storage, or multiple codegen units. These areas remain unmeasured. This record does not classify them as supported or unsupported.
