#!/usr/bin/env bash
# Compare official RISC-V generation with the two upstream event interfaces.
# A reproduced interface mismatch is not a completed coverage result.
set -euo pipefail

fail() {
  printf 'event generation error: %s\n' "$1" >&2
  exit 1
}

require_revision() {
  [ "$(git -C "$1" rev-parse HEAD)" = "$2" ] || fail "wrong revision: $1"
  [ -z "$(git -C "$1" status --porcelain)" ] || fail "changed source: $1"
}

[ "$#" -eq 2 ] || fail 'usage: generation.sh SAIL_SOURCE RISCV_SOURCE'
readonly sail_source=$1
readonly riscv_source=$2
require_revision "$sail_source" 5745ea9e5369ab4fc51de6f8b773dd8ebc323357
require_revision "$riscv_source" abeec0f2eb20b5508b756c37e7274a7e5919ac15
readonly build_dir="$sail_source/_build/default/src"
readonly sail_bin="$build_dir/bin/sail.exe"
readonly config="$riscv_source/build/config/rv64d_v256_e32.json"
readonly config_digest=ebd1ca3de6444ee673cc4a4e2d30a978edb2dec02b9da1768495c6e10b48daf0
[ -x "$sail_bin" ] || fail "missing compiler: $sail_bin"
[ -f "$config" ] || fail "missing configuration: $config"
[ "$(shasum -a 256 "$config" | awk '{print $1}')" = "$config_digest" ] ||
  fail 'configuration digest differs'
readonly records=$(mktemp -d /tmp/hyperray-event-generation.XXXXXX)
printf 'Generation records: %s\n' "$records"
arguments=()
for backend in lean lem coq c; do
  plugin="$build_dir/sail_${backend}_backend/sail_plugin_${backend}.cmxs"
  [ -f "$plugin" ] || fail "missing compiler plugin: $plugin"
  arguments+=(--plugin "$plugin")
  shasum -a 256 "$plugin" >> "$records/digests.txt"
done
shasum -a 256 "$sail_bin" "$config" >> "$records/digests.txt"
"$sail_bin" --version > "$records/compiler.txt"
arguments+=(--strict-var --strict-bitvector --strict-exponentials)
arguments+=(--memo-z3-path "$records/sail.memo" --config "$config")
arguments+=(--lean --lean-output-dir "$records" --lean-noncomputable)
arguments+=(--lean-non-beq-type instruction --lean-non-beq-type ExecutionResult)
arguments+=(--lean-non-beq-type Step)
arguments+=(--lean-import-file ../handwritten_support/RiscvExtras.lean)
cd "$riscv_source/model"
if ! env SAIL_DIR="$sail_source" "$sail_bin" "${arguments[@]}" \
    -o Lean_Control --all-modules riscv.sail_project > "$records/control.log" 2>&1; then
  fail "control generation failed: $records/control.log"
fi
[ -s "$records/Lean_Control/LeanControl.lean" ] || fail 'control output is absent'
if env SAIL_DIR="$sail_source" "$sail_bin" "${arguments[@]}" \
    -D CONCURRENCY_INTERFACE_V2 -o Lean_Events --all-modules riscv.sail_project \
    > "$records/events.log" 2>&1; then
  fail 'the expected interface mismatch did not occur; inspect the new result'
fi
rg -F "Unknown outcome variable 'pa in instantiation" "$records/events.log" ||
  fail "unexpected generation error: $records/events.log"
printf 'Control generated; event interface mismatch reproduced.\n'
