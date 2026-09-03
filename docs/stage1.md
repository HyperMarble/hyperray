# Stage 1 — EXTRACT (Rust, on `rustc_public`)

Status: PROPOSED 2026-09-03. Replaces the Charon half of `design.md` §3 for
Rust. Written after the probe in "Measured today"; no line here is from
memory. C++, Go and Python keep their own front ends (`design.md` §3 table).

## 1. What it is

Stage 1 answers one question: **for every function this patch changes, what
does the compiler say about it?** Name, file, first and last line, and
whether it has a body.

It does that in two passes. Pass 1 reads the patch text alone and learns
which files and which line ranges changed. Pass 2 compiles the crate and
asks the compiler for every item it built, then joins the two by file and
line. Nothing is matched by name, because tools spell names differently and
line numbers are the same in both.

Until today pass 2 asked Charon. Charon is a re-encoder: it reads rustc's
MIR and writes its own JSON, and it refuses what it has no case for — on
noodles-util, 36 functions with `"Coroutines are not supported"`, 2 with a
`Hax panicked` crash, 122 whose body it never opened. `rustc_public` is not
a re-encoder; it is the compiler's own published API, the same one Kani is
built on (64 files in `kani-compiler/src`, `main.rs:38`). What rustc
compiles, it hands over. The probe below shows 0 refusals where Charon had
36.

## 2. Input

- `solution.patch` — a unified diff.
- The base checkout it applies to, already `cargo build`-able.
- Nothing else. The task text is read at stage 5 only (`design.md` §2).

## 3. Output

One file, `manifest.json`, same shape as today plus one field:

```json
{
  "functions": [
    { "path": "noodles-util/src/variant/io/reader.rs",
      "name": "noodles_util::variant::io::reader::Reader::read_record",
      "start_line": 100, "end_line": 118,
      "text": "pub fn read_record(&mut self …",
      "status": "Extracted",
      "answered_by": "rustc_public" }
  ],
  "globals": [ { "path": "…", "line": 18, "source_text": "const MAX…" } ],
  "opened":  [ { "path": "…", "reason": "named by hunk 3" } ]
}
```

`status` is `Extracted | Missing | FileNotSeen`. **`Refused` is gone.** Under
`design.md` §2 a stage answers every row; a front end that cannot read a body
is our routing bug, not a result. If `rustc_public` ever returns no body for
a patched function, the stage fails loudly with the item's name — it does not
write a row saying so.

`answered_by` names the reader that produced the row. One value today
(`rustc_public`); the field exists so a second reader can never be silent
about which one spoke.

## 4. Who knows what

Every fact below was read from the toolchain source on disk at
`$TOOLCHAIN/lib/rustlib/rustc-src/rust/compiler/rustc_public/src`,
toolchain `nightly-2026-08-21`.

| fact | call | source |
|---|---|---|
| every item the crate built | `all_local_items() -> CrateItems` | `lib.rs:301` |
| is it a fn, static, const | `item.kind() -> ItemKind` = `Fn \| Static \| Const \| Ctor` | `lib.rs:178` |
| its full path | `item.name() -> Symbol` (`= String`) | `crate_def.rs:54`, `lib.rs:93` |
| does it have a body | `item.body() -> Option<Body>` | `lib.rs`, `CrateItem::body` |
| where it is | `body.span.get_filename()` | `ty/tys.rs:279` |
| first/last line | `body.span.get_lines() -> LineInfo{start_line,end_line,…}` | `ty/tys.rs:284,305` |
| its blocks (stage 3) | `body.blocks: Vec<BasicBlock>` | `mir/body.rs:15` |
| where a block jumps (stage 3) | `terminator.successors() -> Vec<BasicBlockIdx>` | `mir/body.rs:196,201` |
| what a terminator is | 10 variants incl. `InlineAsm` | `mir/body.rs`, `TerminatorKind` |

`item.span()` is the **signature** span, not the function: measured
2026-09-03, a function spanning lines 2-10 reported `2-2`. Phase C joins by
line range, so that span would miss nearly every patched function. The field
to read is `body.span`, documented at `mir/body.rs:37` as "the span that
covers the entire function body". An item with no body (a `const`) has no
`body.span`, and only there does `item.span()` apply.

The adapter holds no rule about Rust. Everything above is a call.

## 5. Shape: a driver plus a reader

`rustc_public` is a library that only works **inside** a running compiler
(`compiler_interface.rs`: every query goes through thread-local state). It
cannot be called from ordinary code. So stage 1 gains one small program.

**D1 (needs your yes).** A new binary `mir-dump` lives in the Hyperray repo
at `tools/mir-dump/`, with its own `rust-toolchain.toml` pinning
`nightly-2026-08-21` and components `rustc-dev, rust-src`. It is ~150 lines
and it is *our* code, which is why it sits in the repo — unlike Kani and
Charon, which stay outside it. It compiles the crate, walks
`all_local_items()`, and writes one JSON file. The adapter stays on stable
Rust and reads that file, exactly as it reads Charon's today.

Running it over a whole crate follows Kani: set `RUSTC` to our driver and let
cargo do the build (`call_cargo.rs:121`, `:239`, with `RUSTC_BOOTSTRAP=1` at
`:243`). On macOS the driver needs `DYLD_FALLBACK_LIBRARY_PATH` set to the
toolchain's `lib` (measured: without it, `dyld: Library not loaded:
librustc_driver`).

## 6. Phases

### Phase A — read the patch (unchanged)

Who knows: the diff format, nothing else.

1. Split the patch into files at `+++ b/`.
2. Split each file into hunks at `@@`; record the added and removed ranges.
3. For each changed file, open it and take the whole `fn` behind every hunk
   by brace depth.

**Test:** for every fixture patch, the file count equals the `+++ b/` count
and each hunk's range lies inside its file's line count.

**Proof:** passes today — `extract/{patch,hunk,locate,function}.rs` are
untouched by this change.

### Phase B — compile once, collect items

Who knows: `rustc_public`.

1. Build the crate with `RUSTC=mir-dump`, `--all-features`.
2. In `after_analysis`, walk `all_local_items()`.
3. For each item write: `name()`, `kind()`, `span().get_filename()`,
   `span().get_lines()`, and whether `body()` is `Some`.
4. Write one file per crate, `target/hyperray/<crate>.mir.json` — per-crate,
   never a shared name (four crates once overwrote one file and 40 functions
   read as `Missing`).

**Test:** every item written has a filename and a line range, and every item
with `kind() == Fn` reports whether it has a body. No item is written twice.

**Proof:** (not built)

### Phase C — join by file and line

Who knows: nothing. Arithmetic on two lists.

1. For each function found in Phase A, find the Phase B item whose file
   matches and whose line range overlaps.
2. One match → `Extracted`. No match → `Missing`. File never compiled →
   `FileNotSeen`.
3. Never compare names. Item paths differ between the patch text
   (`read_record`) and the compiler
   (`noodles_util::…::Reader::read_record`); line numbers do not.

**Test:** on every fixture with a source tree, every patched function has a
status, no patched file is `FileNotSeen`, and every opened file is a file
the patch names.

**Proof:** (not built)

### Phase D — async items are child items

Who knows: rustc's own naming.

An `async fn f` compiles to two items: `f`, whose body is one block that
builds a future, and `f::{closure#0}`, whose body is the state machine with
the real loop. Measured today: `t::async_loop` had 1 block and 0 back edges;
`t::async_loop::{closure#0}` had 15 blocks and 6 back edges.

1. When an item's name ends in `::{closure#N}`, record its parent name too.
2. The join in Phase C uses the span, so both land on the same source
   function without any name parsing.
3. Stage 3 reads the child's blocks, not the parent's.

**Test:** for every item whose name ends in `{closure#N}`, its span lies
inside its parent's span.

**Proof:** (not built)

## 7. Build order

1. `tools/mir-dump` — the 4 fields of Phase B, one JSON file. (D1 first.)
2. `extract/mir.rs` — serde structs for that file, replacing `ullbc.rs`.
3. `extract/seen.rs` — swap the reader; `join.rs` unchanged.
4. Delete `extract/charon.rs`, `refusal.rs`, `ullbc.rs`; drop `Refused` from
   `Status`.
5. Re-run the stage 1 tests on every fixture.

Phase A files (`patch, hunk, locate, function, names, span, workspace`) are
not touched. That is 361 of the current 770 lines staying as they are.

## 8. Measured today

Probe: `hyperray-work/smir-probe`, 60 lines, run on a 4-function file with a
plain loop, an `async fn` with a loop, a `thread::spawn`, and an `asm!`.

```
BODY blocks=6  back=1  asm=false  t::plain_loop
BODY blocks=1  back=0  asm=false  t::async_loop
BODY blocks=15 back=6  asm=false  t::async_loop::{closure#0}
BODY blocks=4  back=0  asm=false  t::spawner
BODY blocks=1  back=0  asm=false  t::spawner::{closure#0}
BODY blocks=2  back=0  asm=true   t::with_asm
SUMMARY with_body=6 no_body=0 with_loop=2
```

Same four shapes under Charon: the async one is `"Coroutines are not
supported"`. Charon at `--mir optimized` gave the same 36 errors; at
`--monomorphize` it dropped generics and emitted 75 of 410 functions. Both
measured dead this morning.

## 9. Not measured, not assumed

Each of these is checked before the line that depends on it is written.

- The driver has run on **one file**, not on a cargo workspace. Phase B step
  1 is unproven until it runs on noodles.
- `-Zalways-encode-mir` exists on this nightly (`rustc -Z help`) and the
  rust-lang forum says it plus `-Zbuild-std` yields dependency bodies. Not
  run. Until it is, whether the 122 opaque bodies come back is unknown.
- Timing on noodles is unknown. Charon takes 37 s for four crates.
- Reading `const` and `static` values through `ItemKind::Const` is not
  tried; today's globals come from source text.
- Whether two crates in one workspace can share a single driver invocation,
  or need one each, is not tested.
