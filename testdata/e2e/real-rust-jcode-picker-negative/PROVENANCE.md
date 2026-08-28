# Provenance: jcode fallback model picker routes

- Repository: `/Volumes/Hak_SSD/jcode`
- Source commit: `f85c2d596f02d943dbb72e45a88e4e6071c9f8b7`
- Parent commit: `95087d57b3d5b5dd02c64a12c44e149d4426abad`
- Source path at both commits: `src/tui/ui_inline_interactive.rs`
- Author: `jeremy <94247773+1jehuang@users.noreply.github.com>`
- Author date: `2026-05-19T00:53:28-07:00`
- Commit subject and complete instruction source: `Mark fallback model picker routes`
- Commit body: empty

The source commit changes one file by 31 insertions and 3 deletions. The split is mechanical: changed hunks wholly above parent line 801 are in `patches/solution.patch`; the changed hunk intersecting the excluded tail beginning at line 801 is in `patches/tests.patch`. The instruction, base metadata, and solution patch are the test-blind task-authoring inputs. Test bytes remain a separately frozen verifier input.

The two patches are ordered as verifier evidence first, solution second. Applying `patches/tests.patch` and then `patches/solution.patch` to the parent tree must reproduce the source commit tree byte for byte. The recorded hashes and the reconstruction command output are evidence of that invariant, not semantic proof.

- `patches/solution.patch` SHA-256: `2bf8146b4dddabd13524784f04acf9bf6057bd17c5af873e567a32c137eac790`
- `patches/tests.patch` SHA-256: `d6b204804ac06e05a8361a90b399b42ef9a565d49f08541d72cfb6060e6ea6f5`

## Byte-exact source and environment snapshots

The bundle retains the complete upstream files needed to locate the changed private function in its actual module context. These snapshots are provenance evidence, not task-authoring inputs.

- Base `source/base/src/tui/ui_inline_interactive.rs`: `bdbc90cc03a2639c6c0641c399c93c03f91ca6f071ceab53c1cffc3f7e064cfa`
- Solution `source/solution/src/tui/ui_inline_interactive.rs`: `cfa56340ecd9d37b071e429e3be3e3c8104b3342be7e03de1ba94869b1cfd95a`
- Shared module context `src/lib.rs`: `a95074a7dfbc4560480feec2747c09a1ba8659d0af7649cfad3e83016e09d1bd`
- Shared module context `src/tui/mod.rs`: `787b1879a06cacadfd6c2ae439cc2d0ecd64e48351fcc7a9813fcf6cfdcae6c1`
- Shared module context `src/tui/ui.rs`: `6258d21757703addb85a3ec4d870a8048875215a6116d08cf1b6caf84a640a2f`
- `environment/Cargo.toml`: `10fa041aea4c2d816c215c6a9f4595dab12b642a5eebfcd3f77430650e7bfa52`
- `environment/Cargo.lock`: `0732e41972891a86886014b65c27bc3f8df291738f952a9623a29708bd16b798`
- `environment/.github/workflows/ci.yml`: `6b024a0193fc885e5f890b13fd2a35b1f8dc0adfcb88ab47ff6e2a8dd9be7ea3`

The upstream Linux/macOS CI build command is preserved verbatim in the workflow and compiles library/binary tests with:

```sh
python3 .github/scripts/run_with_timeout.py 900 \
  "$(rustup which cargo)" test --target "${target}" --lib --bins --no-run
```

The focused native unit-test invocation is:

```sh
cargo test --locked --lib tui::ui::inline_interactive_ui::tests::fallback_route_details_are_warning_limited -- --exact
```

Ray must translate the real reference and the real embedded Rust verifier independently. A wrapper, copied function, generated replacement verifier, or sampled subset of the eight Boolean inputs is not accepted.

## Verifier evidence discovered after the test-blind split

The parent source already contains `picker_row_marker_uses_explicit_unavailable_marker`. In the reconstructed solution it asserts `picker_row_marker(true, false, true) == "▸"` at line 959. The source commit adds `fallback_route_details_are_warning_limited`, which asserts the same call equals `"⚠"` at line 812, and the solution returns `"⚠"`. The faithful verifier is therefore internally contradictory for that assignment. This is frozen test evidence; it is not used to author the requirements.

The corrected v0.10 architecture consequently expects either an exact false-negative witness from the real verifier or `PROOF BLOCKED` if the full module/test translation is unsupported. It must never report `VERIFIED` for this reconstruction.
