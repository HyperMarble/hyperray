# ARM v9 model artifact

`patches/arm-v9-model-v1.patch` ports the ARM memory admission to the shape
of the sail-arm v9.x models. It applies on top of the execution-guards tree
(the one that built the 19 Sep release trio; `arm_memory.rs` there is
byte-identical to `/Users/hak/hyperray-isla-fp-task-a-20260907`).

## What changed

The v8.5 and v9.4 models spell the same memory path with different names
and shapes. The patch binds each role by the spelling the loaded model has,
chosen once at `ArmMemorySchema::resolve` from whether the model declares
`zaget__Mem` (v8.5) or `zMem_read` (v9).

| role | v8.5 | v9.4 |
|---|---|---|
| parent read / write | `aget_Mem` / `aset_Mem` | `Mem_read` (+`__1`, `__2`) / `Mem_set` (+`__1`, `__2`) |
| single read / write | `AArch64_aget_MemSingle` / `aset_MemSingle` | `AArch64_MemSingle_read` (+`__1`) / `MemSingle_set` (+`__1`) |
| physical read / write | `aget__Mem` / `aset__Mem` | `PhysMemRead` / `PhysMemWrite` |
| descriptor argument order | (desc, translation, size, access) | (desc, size, access, translation) |
| access type enum | `AccType` {NORMAL, VEC, IFETCH, VECSTREAM} | `AccessType` {GPR, ASIMD, IFETCH, TTW, SVE} |
| access type in parent call | enum argument | `AccessDescriptor.acctype` |
| page-table walk marker | `AccessDescriptor.page_table_walk` + `AArch64_TranslationTableWalk` scope | `AccessType_TTW` on the walk's descriptor, no separate scope |
| walk translation | `None` | `Some(TranslationInfo)` |
| physical address | `FullAddress{NS: bits(1), address: bits(52)}` | `FullAddress{paspace: PASpace, address: bits(56)}` |
| fault status field | `FaultRecord.typ` | `FaultRecord.statuscode` |
| memory type field | `MemoryAttributes.typ` | `MemoryAttributes.memtype` |
| shareability | `shareable`, `outershareable`: bool | `shareability`: Shareability {NSH, ISH, OSH} |
| tagging | `tagged`: bool | `tags`: MemTagType |
| system registers | `bits(64)` / `bits(32)` | `struct { bits: bits(64) }`, TTBRs `bits(128)` |
| table base | `TTBR0_EL1` | `_TTBR0_EL1` plus banked `TTBR0_NS` / `TTBR0_S` |
| instruction fetch (boundary hook) | `__fetchA64 : unit -> bits(32)` | `__FetchInstr : bits(64) -> (enc, bits(32))` |
| device windows | none reached | `PhysMemRead` tests GIC/UART windows before the primitive |

Three v9-specific admissions, each recorded in a comment at the site:

- Overloaded entries re-enter the same scope (`Mem_read` inside
  `Mem_read__2`). The inner call is matched to the open scope by address
  and size rather than opening a second one.
- The walk descriptor is built from walk state, so its fault record and
  cache hints are left undefined by the model. Those two checks are skipped
  for a walk read; type, tags, shareability, security, and width are still
  checked.
- The descriptor's physical address is asserted into the declared ranges at
  descriptor entry, not only at the primitive. `PhysMemRead` decides whether
  an address is a device window before reaching the primitive, so the
  primitive check alone comes too late to exclude that path.

`isla-litmus-dump` moves from `B64` to `B129` because v9 declares 128-bit
registers. The other two tools already used `B129`.

## Build

```sh
git apply --check patches/arm-v9-model-v1.patch
git apply patches/arm-v9-model-v1.patch
env CARGO_BUILD_JOBS=2 cargo test --locked --offline -p isla-lib
env CARGO_BUILD_JOBS=2 cargo build --locked --offline --release --bin isla-axiomatic --bin isla-litmus-dump --bin isla-footprint
```

## Model and config

- model: `isla-snapshots/armv9p4.ir` (sha256
  `02918f8141ca06d78aff955c3c15c9f2d1c04df24e00176d7e4b6132cd8c75ec`),
  decompressed from the upstream `armv9p4.ir.gz`.
- config: upstream `configs/armv9p4.toml` with only the three toolchain
  paths changed, as was done for v8.5.

## Measured

On 19 Sep with the v9.4 trio, `t_u64` (`x.wrapping_add(22)`, claim
`f(20) == 42`) is PROVED through the same three-tool path as v8.5. The
v8.5 model failed `ucvtf`/`fcvtzu` on an uninitialised `part` in
`aarch64_float.sail:1761`; v9.4 initialises it. v9.4 in turn reaches
`FixedToFP` and `Reduce`, which are `Unimplemented` in that model.

The v8.5 path is unchanged: the same binary resolves v8.5 by name and takes
the old branches.
