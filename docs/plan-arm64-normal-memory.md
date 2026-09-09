# ARM64 finite Normal-memory execution plan

Status: source-derived design, 2026-09-09. The proposed configuration and memory
profile have **not been executed or validated by this investigation**. No native
builds or runtime tests were performed. This document records implementation
contracts and required evidence, not a completed capability.

This follows [native execution M1/M2](plan-arm64-native-execution.md) and
[first-execution acceptance](plan-arm64-first-execution-acceptance.md).
The full target remains original ARM64 Mach-O execution with compatible data
semantics. Fetch-only and aligned scalar access through finite tables are
internal milestones. Neither completes general Darwin VM, the macOS ABI, process
runtime, system calls, dynamic loading, or native 16K-page behavior.

## Sources and identity

Native paths below are relative to:
`/Users/hak/hyperray-isla-fp-task-a-20260907`.
This isolated tree has no Git metadata and contains concurrently changing work.

- Selected IR: `/Volumes/Hak_SSD/hyperray-research/isla-snapshots/armv8p5.ir`.
- Selected baseline config:
  `/Volumes/Hak_SSD/hyperray-research/isla-7f6882b/configs/armv8p5.toml`.
- Existing MMU example in the native tree: `configs/armv8p5_mmu_on.toml`.
- Main repository: `/Volumes/Hak_SSD/hyperray`.

Line numbers are inspection references, not revision identities. Acceptance must
bind actual IR/config/native binary hashes, original Mach-O hash and slice,
query, effective state after reset, profile version, mappings, physical backing,
and concrete table contents. Source compatibility does not validate an existing
binary or grant it a new capability.

## 1. Keep the first execution milestone separate

`memory_profile = "arm64-fetch-only-v1"` admits the original eight code bytes
`00 04 00 91 c0 03 5f d6` at `[0x1000002e8, 0x1000002f0)`.
The fixture is `fixtures/machine/arm64/tiny-arm64-static`, with SHA-256
`a55e2554b91b1a5d451df1b1125a883f9f239f610ba841caed55a75651574ac4`.
The production policy must use declared executable ranges, not these constants.

This profile admits two actual four-byte instruction fetches. It rejects every
data descriptor, including Normal data, and rejects table walks. A native
continuation boundary stops before a third `__fetchA64` body. With `R0 = 3`, the
terminal result is `R0 = 4`. Caller SP must be applied after reset. A correct
negated assertion is UNSAT and a deliberately wrong assertion is SAT.

Admission requires a live `zaget__Mem` descriptor and its matching primitive
request. Missing, stale, returned, reused, or cross-fork permits are errors.
Validation occurs before initialized-byte, custom-region, and array fast paths.
An absent `TranslationInfo` does not prove Normal attributes or code permission.
Readonly loaded bytes do not establish executable permission.

## 2. Why real translation is required for Normal data

In the selected IR:

- `zHasS2Translation`, near 28828, depends on EL2 enablement, host state and EL.
- `zAArch64_TranslateAddressS1Off`, near 32439, normally produces Device nGnRnE
  for data with stage one disabled. Its Normal-data alternative needs the
  applicable stage-two regime and `HCR_EL2.DC`.
- `zAArch64_SecondStageTranslate`, near 35922, enables translation when
  **`HCR_EL2.VM || HCR_EL2.DC`**, not only when VM is set.

Setting DC while clearing VM therefore does not provide a no-walk Normal-data
shortcut. Do not replace descriptors or change attributes in the memory callback.

Device nGnRnE is not generally equivalent to Normal memory. In particular,
unaligned Device accesses fault even when SCTLR.A is clear. A separately labeled,
synchronous, single-hart Device-RAM environment could be analyzed, but it is not
assigned for implementation and must not substitute for the macOS data target.

## 3. Reusable public table support and its limits

`isla-axiomatic/src/page_table/setup.rs` exposes:

```rust
pub fn armv8_litmus_page_tables<B: BV>(
    memory: &mut Memory<B>, litmus: &Litmus<B>, isa_config: &ISAConfig<B>,
) -> Result<PageTableSetup<B>, SetupError>;

pub fn armv8_page_tables<B: BV>(
    memory: &mut Memory<B>, vars: HashMap<String, TVal>, num_threads: usize,
    constraints: &[Constraint], isa_config: &ISAConfig<B>,
) -> Result<PageTableSetup<B>, SetupError>;
```

The CLI has `--armv8-page-tables`. Litmus TOML has `page_table_setup`, parsed by
`page_table/setup_parser.lalrpop`. `doc/translation.adoc` documents the existing
4K mapping support. The DSL includes explicit S1 tables, definite mappings,
`option default_tables = false`, and `option self_map = false`.

`PageTableSetup` carries `memory_checkpoint`, `all_addrs`, `physical_addrs`,
`initial_physical_addrs`, named `tables`, and `maybe_mapped`. Reusable storage is
`PageTables::{new, alloc, map, identity_map, freeze}` in `page_table.rs`.

The generic setup is not already a sound finite sequential profile:

- Default setup creates both S1 and S2 tables and maps tables into tables.
- `map_code`, near setup line 1096, maps `isa_config.thread_base + i * page_size`,
  not original Mach-O sections. Nested explicit tables also invoke this helper.
- `VirtualAddress::from_u64` silently clears bits above 47.
- `PageTables::alloc/range` has no checked capacity, base alignment, or overflow
  contract. `map/update` can overwrite existing leaf mappings.
- `Memory::add_region` simply appends a region. It does not prove disjointness.
- Setup uses `translate(...).unwrap_or(0)` for `physical_addrs`.
- `initial_physical_addrs` feeds axiomatic initial writes. It does not initialize
  the sequential byte array.

Add a checked finite builder using the existing table representation, or a
strict setup mode with explicit image pages and no automatic mappings. Preserve
existing axiomatic behavior outside the new profile. Do not inherit phantom
thread-base pages, self mappings, failed-translation-to-zero fallbacks, optional
mappings, or symbolic descriptor choices.

The IR implements `_TLB : fvec(1024, TLBLine)`, `zTLBLookup` near 31397 and
`zTLBCache` near 31594. A hit can return a cached descriptor with no translation
metadata. No ready public preinstalled-TLB setup was found in the inspected
page-table/config sources. Constructing cache contexts and permissions is not a
smaller demonstrated solution. Keep `__tlb_enabled = false` and use real walks.

## 4. Proposed finite profile and physical backing contract

The following identifier and metadata are **proposed**, not existing accepted
TOML or a measured capability:

```text
memory_profile: "arm64-normal-s1-fixed-4k-v1"
table_base: u64
table_capacity_pages: u32
mappings: [{ va: u64, pa: u64, length: u64, permission: RX | R | RW }]
backing_ranges: explicit initialized or declared symbolic physical RAM
executable_byte_ranges: original executable byte extents
identity: model/config/binary/post-reset-state/mapping/backing/table digests
```

Implement a typed schema with unknown/missing-field errors. The initial finite
profile requires VA = PA, low-48-bit addresses, no aliases, and definite 4K leaf
mappings. Do not truncate addresses to satisfy these requirements.

1. Preserve original Mach-O bytes, virtual addresses, entry, relocations already
   represented by the loader contract, and section permissions. An ELF carrier
   does not license a Linux rebuild or a different program.
2. Initialize physical RAM at the same addresses for identity mapping. A later
   nonidentity profile needs explicit VA-to-PA transport and initialization at
   PA, while PC, pointers and branches retain original VA.
3. Check nonempty page-aligned mapping lengths/bases, checked end arithmetic,
   supported VA/PA width, and coverage of entry and declared data/stack ranges.
4. Declare the backing of page padding. Either explicitly initialize it, declare
   symbolic RAM, or reject access to it. Do not invent zero bytes or unconstrained
   fallback storage. Every admitted access must be backed over its full extent.
5. Reject W+X, incompatible permissions for sections sharing a page, conflicting
   duplicate VA mappings, and table/RAM/thread-reservation collisions. Page
   permission and executable-byte permission are separate checks.
6. Use a disjoint physical table arena. For N distinct 4K leaf pages, `1 + 3*N`
   tables is a conservative preallocation bound for a four-level tree. Check the
   arithmetic, capacity, allocation count and final range before region insertion.
7. Require concrete descriptors. Reject block mappings, raw descriptor overrides,
   `?->`, wildcard attributes, dynamic remapping, and mutable page tables.

For each S1 leaf, explicitly set `AF=1`, `DBM=0`, `Contiguous=0`, `nG=0`,
`AttrIndx=000`, `SH=00`, and `Valid=1`. Use AP=11 and UXN=0 for RX, AP=11 and
UXN=1 for R, AP=01 and UXN=1 for RW. Pin PXN=1 for this EL0-only profile. Table
pages need not be user-VA mapped. Relevant descriptor bit positions are in
`S1_PAGE_ATTR_FIELDS`: UXN54, PXN53, Contiguous52, DBM51, nG11, AF10, SH9:8,
AP7:6, NS5, AttrIndx4:2, Valid0. Pin NS consistently with the selected regime.

## 5. Source-derived state after reset, not an executed recipe

The existing MMU example uses `TCR_EL1 = 0x0000000080000010`. Its comment describes
48-bit input, but IPS[34:32] is 000, which selects **32-bit physical output**.
It cannot identity-map the original fixture at `0x1000002e8`.

The selected translation walk reads IPS from TCR_EL1[34:32] near IR 61248 and
decodes 000 as 32 and 101 as 48 near 62845-62935. It clamps output size to PAMax.
The default `Maximum Physical Address Size` implementation returns 52 near 8945.

A minimal S1-only candidate, derived from this source, is:

| State | Required candidate value or condition |
| --- | --- |
| PSTATE | EL0, nRW=0, PAN=0, SP=0 |
| SCTLR_EL1 | `0x0000000004800005`: M=1, C=1, I=0, A=0, EE=0, E0E=0 |
| TCR_EL1 | `0x0000000580000010`: existing example plus IPS=101 |
| TTBR0_EL1 | Declared aligned S1 root, ASID=0, CnP=0 |
| MAIR_EL1 | `0xffffffffffffffff`, with leaf AttrIndx=000 |
| HCR_EL2 | `0`, including VM=0, DC=0, E2H=0, TGE=0 |
| SCR_EL3 | `0x00040101`, from the example reset, including NS=1 |
| Model identity | Selected no-AArch32 CFG EL0..EL3 values=1 and exact feature set |
| External effects | TLB, MTE, MPAM and trickbox disabled, no timer/MMIO backing |

TCR has T0SZ=16, TG0=4K, SH0=ORGN0=IRGN0=0, HA39=0, HD40=0 and TBI0=0.
The admitted VA domain uses TTBR0 only. Do not admit the upper TTBR1 regime.
Require effective PAMax at least 48. This example is not a general Darwin control
state and has not passed a native execution gate in this investigation.

The zero HCR is not an attribute override: the selected second-stage function
returns its input S1 descriptor unchanged when VM and DC are both zero.
`HasS2Translation` may still call that passthrough. In this nonsecure S1-only
path, the passthrough returns `None` translation metadata, discarding the S1
metadata. Validate the real descriptor rather than inferring attributes from
that `None`.

Apply and verify these values after `zmain` calls `zinit` and
`sail_reset_registers`, before the first real fetch. Defaults before reset are
not proof of effective state. Bind the final SP and continuation too. A mismatch
or unsupported control mutation is an error, not a new solver assumption.

## 6. Exact model schema and three access classes

Resolve symbols and signatures from `SharedState.symtab`, `functions`, and
`type_info.{structs,enums,unions,enum_members}`. Do not use numeric symbol IDs or
formatted enum strings.

Core IR symbols:

- `zaget__Mem(AddressDescriptor, option<TranslationInfo>, %i, AccessDescriptor)
  -> %bv`, near 58784.
- `zaset__Mem` has the same arguments followed by `%bv`, returning unit, near 58318.
- `zAddressDescriptor`: `zfault`, `zmemattrs`, `zpaddress`, `zvaddress:%bv64`.
- `zFullAddress`: `zNS:%bv1`, `zaddress:%bv52`.
- `zFaultRecord.ztyp`: require `zFault_None`. Other no-fault payload fields are inactive.
- `zMemoryAttributes`: `ztyp`, `ztagged`, `zdevice`, `zinner`, `zouter`,
  `zshareable`, `zoutershareable`.
- `zMemAttrHints`: `zattrs:%bv2`, `zhints:%bv2`, `ztransient:bool`.
- `zAccessDescriptor`: `zacctype`, `zmpam`, `zpage_table_walk`, `zlevel`,
  `zsecondstage`, `zs2fs1walk`.

All admitted classes require no fault and Normal, untagged memory. The following
active-field masks apply to the exact candidate state and SH=00 leaves above:

| Class | Descriptor and primitive | Inner and outer | Shareable / outershareable |
| --- | --- | --- | --- |
| Actual instruction | page_table_walk=false, IFETCH, size4 | attrs00, hints00, transient inactive | true / true |
| Actual scalar data | page_table_walk=false, NORMAL, size1/2/4/8 | attrs11, hints11, transient=false | false / false |
| S1 table read | page_table_walk=true, secondstage=false, level0..3, size8 | attrs00, hints00, transient=false | true / true |

The values follow `zLongConvertAttrsHints` near 27307,
`zAArch64_S1AttrDecode` near 33112, `zShortConvertAttrsHints` near 26317,
`zWalkAttrDecode` near 26423, and `zMemAttrDefaults` near 9579.
Normal `zdevice` is inactive and explicitly undefined. Under I=0, the instruction
cache-disabled branch leaves transient undefined. The PTW short decode sets
transient=false even for NC. Do not require all undefined fields to be concrete.
A symbolic active field can be accepted only when its required value is entailed,
not merely satisfiable. Never add constraints to manufacture admission.

Ordinary `zCreateAccessDescriptor` initializes acctype, mpam and
page_table_walk=false. Its level/secondstage/s2fs1walk are inactive.
`zCreateAccessDescriptorPTW`, near 30304, preserves the original acctype and sets
page_table_walk=true, secondstage and level. It assigns secondstage twice and
leaves s2fs1walk undefined. Check the enclosing walk's s2fs1walk argument instead
of demanding a concrete value for that unset field.

**A fetch PTW is an IFETCH-kind eight-byte read, not an instruction fetch.**
The actual walk near IR 63396 calls `zaget__Mem` with this descriptor. The
primitive can therefore have `opts.is_ifetch=true`. A data PTW can similarly
have ordinary explicit access kind. The descriptor VA is the original translated
VA, not the address of the PTE. Only a live PTW descriptor whose aligned PA lies
in the verified table arena may access that custom region. It must not be routed
through executable byte ranges or the RAM array.

The next scalar milestone is explicitly aligned. Scope
`zAArch64_aget_MemSingle(%bv64, %i64, AccType, bool) -> %bv` and
`zAArch64_aset_MemSingle` with an extra `%bv` value, near 59285 and 350551.
Require the original `wasaligned=true` for actual data. Checking only the final
byte-size descriptor would miss model-generated unaligned splitting. Later
unaligned and 16-byte support needs explicit child-access and partial-fault tests.

## 7. Request pairing and fast-path enforcement

Read request type:
`zMem_read_requestzIUarm_acc_typezIzKzCbzCOzIRTranslationInfozKzK`.
Fields are access_kind, pa, sizze, tag:bool, translation, and va:option<bv>.

Write request type:
`zMem_write_requestzIUarm_acc_typezIzKzCbzCOzIRTranslationInfozKzK`.
Fields are access_kind, pa, sizze, tag:option<bit>, translation, va:option<bv>,
and value:option<bv>. Resolve the encoded field names with their `z` prefixes.
The PA field is generic `%bv` in the schema but is 56 bits on this model path.

- Ordinary kind is `zAK_explicitzIUarm_acc_typezIzKzK` containing
  `zExplicit_access_kind{zvariety=zAV_plain,zstrength=zAS_normal}`.
- Fetch kind is `zAK_ifetchzIUarm_acc_typezIzKzK(unit)`.
- Require read tag=false or write `zNonezIozK(unit)` and no primitive tag.
- Require VA `zSomezIbzK` matching the descriptor VA64. For writes require value
  `zSomezIbzK` matching the descriptor value and primitive data, with width 8*size.
- Match descriptor PA52, zero-extended request PA56 and primitive PA64 exactly.
- Match direction, size, options, live descriptor class and translation option.
  `None` is expected on the precise S1-only path, not an attribute certificate.

Extend clone-local, scoped one-use permits in `arm_memory.rs` through
`Memory::enter_model_call` and `leave_model_call`. Scope table provenance to
`zAArch64_TranslationTableWalk(%bv52,%bv1,%bv64,AccType,bool,bool,bool,%i)`.
Validate secondstage=false and s2fs1walk=false in that invocation. A permit pairs
its descriptor invocation with the next exact low-level operation. Clone state
with the execution path, consume once and invalidate on return. Reject bypasses
through function assumptions or abstractions. Do not use a global latest permit.

Run the guard before `read_initialized`, custom dispatch, permission shortcuts,
loaded writes and symbolic callbacks. Do not consume the same permit twice.
Reject Device, atomics including AtomicRMW with exclusive=false, ordered,
architectural, vector, tag and cache-maintenance requests. Reject table writes,
even same-value writes, before any custom-region operation. Tag primitives and
platform aliases need the same fail-closed policy.

Ordinary `__WriteMemory` discards the successful write payload. Do not introduce
conditional stores based on an invented Boolean success condition. Preserve
actual model faults and abort paths rather than filtering them from proofs.

## 8. Setup order, table storage and sequential events

The existing `run_litmus_setup` flow validates the profile, reserves thread
regions, runs table setup, writes raw code, initializes loaded sections, installs
memory callbacks, and creates reset tasks. Integrate the strict builder into
this flow with explicit checks:

1. Resolve model schema and typed profile. Check all image, RAM and arena ranges.
2. Construct and validate concrete tables before adding custom regions. Do not
   add phantom thread-base mappings. Carry the setup solver checkpoint forward.
3. Initialize original physical section bytes and declared RAM before installing
   `SequentialMemory::new(memory.initialized_bytes(), solver)`.
4. Keep table storage separate from that byte array. Never shadow the custom
   table region with a second copy of initialized bytes.
5. Install the descriptor policy and sequential RAM callback. Apply and verify
   state after reset, then execute the actual model.

`ImmutablePageTables::read` near `page_table.rs:971` requires aligned size8 and
returns the descriptor. `write` near 1011 does not mutate the table. It permits
an equality-compatible descriptor event, which is an axiomatic interpretation,
not sequential write forwarding. Thus this profile requires concrete immutable
tables, AF pre-set, DBM clear and hardware AF/dirty updates disabled.

`CustomRegion::read` currently lacks `ReadOpts`. The table backend emits
`ReadOpts::default()` although a fetch PTW reached it with IFETCH options.
Preserve this distinction explicitly. A backward-compatible `read_with_opts`
entry point can carry the real options, or an authoritative typed provenance
event can accompany the legacy event. Do not relabel the request as AK_ttw to
hide the inherited kind. Table read events must remain observable and must not
be counted as executed instructions.

Extend all profile dispatch points deliberately, including `sequential_setup`,
`sequential_candidate`, forbidden calls and `run_litmus` SMT output. Retain the
exclusions for CAT/RF enumeration, extra SMT filters, translation merge/removal,
mutable tables, self modification, interrupts and unrelated custom devices.
Do not enable generic page-table effects by deleting the old sequential gate.

## 9. Required acceptance gates

No gate below was run by this design investigation.

### G0: preserve the fetch-only bridge

- Run the exact original add+ret fixture with unchanged addresses and bytes.
  Observe two size4 code reads and terminal before a third fetch.
- Prove the correct negated assertion UNSAT and a wrong assertion SAT, with
  authoritative PC/R0/SP evidence and all source/input identities.
- Reject every data descriptor in fetch-only, even otherwise valid Normal data.
- Accept inactive undefined fields. Reject active fields whose required values
  are not entailed, without narrowing the input/path domain.
- Reject missing, stale, reused, returned and cross-fork permits, request width,
  VA/PA/value mismatches, tag paths and all unsupported fast-path bypasses.

### G1: finite builder and effective state

- Record effective post-reset controls, SP, model features and schema identity.
- Independently check expected root/PTE bytes, permissions, levels, AF=1, DBM=0,
  invalid unmapped entries, capacity and disjoint physical backing.
- Reject conflicting maps, VA truncation, unsupported PA width, overflow,
  W+X, incompatible shared-page permissions and table/loaded/thread collisions.
- Reject symbolic/maybe descriptors and fallback-to-zero translations.
- With published IPS000 and identity PA above 4GiB, observe an explicit profile
  refusal or native AddressSize fault. Never wrap to low physical bytes.

### G2: actual original fetches through tables

- Run the original add+ret through real S1 walks with TLB disabled.
- Observe size8 reads at levels 0..3 separately from each size4 code fetch.
  Check PTE PA versus original descriptor VA and unchanged original code bytes.
- Observe no table writes and no S2 translation-table-walk invocation. Calls to
  the disabled second-stage passthrough are permitted and are not S2 walks.
- Reject forged PTW provenance and code/data accesses to the table arena.
- Keep table events visible without counting them as instructions. Retain the
  correct-UNSAT/wrong-SAT assertion controls and authoritative terminal evidence.

### G3: real ordinary Normal loads and stores

- Execute original ARM64 load/store instructions for sizes 1, 2, 4 and 8.
- Cover readonly initial loads, writable store-to-load forwarding, SP-relative
  RAM and mixed-size overlapping stores/loads with little-endian reconstruction.
- Preserve symbolic stored values, branching and declared address domains.
  Do not select arbitrary equalities or discard unsupported/error paths.
- Observe genuine Normal WB attrs11/hints11/transient=false descriptors for the
  stated candidate, correct physical backing, and no Device substitution.
- Check correct expected terminal results with UNSAT and wrong results with SAT.

### G4: faults, updates and excluded accesses

- Check invalid/unmapped PTEs, RO writes and NX execution. Preserve actual model
  abort/error evidence instead of reporting those paths as successful proofs.
- With AF=0 and HA=0, check the architectural access-flag fault path. Attempted
  AF/dirty AtomicRMW operations must fail before custom table reads or writes.
- Reject unaligned data explicitly in the aligned milestone at MemSingle scope,
  not after the model hides it through byte splitting.
- Reject range-end crossings without full backing and unsupported tags, atomics,
  ordering classes, vector accesses and cache maintenance before fast paths.
- Later unaligned/size16 stages must test each split child, page boundaries,
  partial stores and subsequent faults without rollback or invented merging.

### G5: integration and honest capability boundaries

- Verify all sequential dispatch points select this profile explicitly. Unknown
  schemas/configs/profiles are errors, not fallback to another semantics.
- Retain faults and events without CAT, RF, extra-SMT or translation-rewrite
  pruning. Never treat a subset of surviving traces as whole-query proof.
- Run RV64 sequential and existing axiomatic table regressions.
- Bind evidence to the actual measured binaries. Label this finite aligned S1
  profile as an internal capability, not complete macOS execution support.

## 10. Logical stages and non-overlapping ownership

These are implementation assignments for the coordinator to authorize after the
first bridge stabilizes. This document does not assign simultaneous native builds.

| Stage / owner | Owned paths and contract |
| --- | --- |
| Existing bridge: Tulip | `isla-lib/src/executor.rs`, frame/task and terminal integration, `isla-axiomatic/src/litmus.rs`, `run_litmus.rs`. No competing edits. |
| Existing and next memory: Palmtree | `isla-lib/src/arm_memory.rs`, `memory.rs`, `sequential_memory/*`, `isla-axiomatic/src/memory_profile.rs`, `sequential_setup.rs`, associated memory tests. |
| A: coordinator-assigned finite-table worker | `isla-axiomatic/src/page_table/setup.rs`, a new checked fixed-table module, `page_table.rs` checked helper APIs and table tests. Coordinate any needed memory trait change with Palmtree. |
| B: Tulip plus Palmtree, disjoint files | Wire the verified fixed-table manifest, setup order and post-reset state. Execute G1 then G2. Keep executor/litmus changes with Tulip. |
| C: Palmtree plus assigned test owner | Add exact ordinary scalar request schema and byte-array forwarding under G3/G4. No generic Normal gate removal. |
| Boundary/evidence: Rose | Go renderer/API schema, original section/permission/physical backing metadata and evidence transport. Coordinate schema before changing native litmus parsing. |

The configuration owner must coordinate the exact derived profile and hash with
Tulip's after-reset integration. The coordinator schedules the only native build
lane. The design author remains available for bounded implementation questions.
Further broad research is not required before these explicit validation stages.
