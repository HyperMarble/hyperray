# Official Integer Proof Slice

This slice proves all 22 operations in five RISC-V RV64 instruction
families. It does not prove the complete machine profile.

The source languages stay separate. The Lean proofs live in `lean/`. The Sail
model fragment lives in `sail/`. The proof commands live in `shell/`.

The source gate accepts only Sail RISC-V commit
`abeec0f2eb20b5508b756c37e7274a7e5919ac15`. It also rejects local changes to
the three selected source files.

The script gets these items from `base_types.sail` and `base_insts.sail`:

- The `uop` catalog and lines 16 through 34 for `UTYPE`.
- The `iop` catalog and lines 143 through 169 for `ITYPE`.
- The `sop` catalog and lines 184 through 211 for `SHIFTIOP`.
- The `rop` catalog and lines 223 through 251 for `RTYPE`.
- Lines 337 through 348 for the single `ADDIW` operation.
- The required shift and subtraction definitions from the model prelude.

The finite harness supplies the RV64 register file and program counter. Sail
then generates all five official execution functions and their decoder.

The Lean proofs establish these results for all finite inputs:

1. All four multi-operation catalogs are exhaustive, and `ADDIW` has one operation.
2. Each generated encoding returns the same instruction through the decoder.
3. Each generated execution function equals its finite state action.
4. Each finite state action uses a value equal to the independent bit circuit.

The five mutation files change output bit zero. The proof command must reject
all five changes and give counterexamples.

Run:

```sh
./proof/sail_addi/shell/run.sh \
  /Volumes/Hak_SSD/sail-riscv \
  /path/to/sail-0.20.2 \
  /path/to/lean-sail \
  /path/to/lean-4.29.0/bin/lake
```

The required Lean support revision is
`79b4d08505af29d88b3918f32d29840fae1fa191`.

The measured axiom reports are:

```text
officialITypeCatalog: [propext]
officialITypeDecode: [propext, Classical.choice, Quot.sound]
officialExecuteITypeBridge: [propext, Classical.choice, Quot.sound]
officialITypeCircuitEquivalence: [propext, Quot.sound]
officialUTypeCatalog: [propext]
officialUTypeDecode: [propext, Quot.sound]
officialExecuteUTypeBridge: [propext, Classical.choice, Quot.sound]
officialUTypeCircuitEquivalence: [propext, Quot.sound]
officialRTypeCatalog: [propext]
officialRTypeDecode: [propext, Classical.choice, Quot.sound]
officialExecuteRTypeBridge: [propext, Classical.choice, Quot.sound]
officialRTypeCircuitEquivalence: [propext, Quot.sound]
officialShiftImmediateCatalog: [propext]
officialShiftImmediateDecode: [propext, Classical.choice, Quot.sound]
officialExecuteShiftImmediateBridge: [propext, Classical.choice, Quot.sound]
officialShiftImmediateCircuitEquivalence: [propext]
officialAddiwDecode: [propext, Classical.choice, Quot.sound]
officialExecuteAddiwBridge: [propext, Classical.choice, Quot.sound]
officialAddiwCircuitEquivalence: [propext]
```

The trust base contains the pinned source, Sail, lean-sail, and the Lean
kernel. The command stores the full proof and mutation output.

The catalog contains the five proved families. Thus, the family-local meaning
result is 5/23, or 21.7%.

This result does not prove the full machine step, traps, memory, concurrency,
the environment model, or the other 18 instruction families.
