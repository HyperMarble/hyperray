#!/usr/bin/env bash
# Reproduce the pinned official-source integer circuit proofs.
# Never accept a source mismatch or a mutation that passes.
set -euo pipefail
fail() {
  printf 'integer proof error: %s\n' "$1" >&2
  exit 1
}
if [ "$#" -ne 4 ]; then
  fail 'usage: run.sh SAIL_RISCV_DIR SAIL_BIN LEAN_SAIL_DIR LAKE_BIN'
fi
readonly source_dir=$1
readonly sail_bin=$2
readonly lean_sail_dir=$3
readonly lake_bin=$4
readonly lean_sail_revision=79b4d08505af29d88b3918f32d29840fae1fa191
readonly script_dir=$(cd "$(dirname "$0")" && pwd)
readonly proof_dir=$(cd "$script_dir/.." && pwd)
[ -x "$sail_bin" ] || fail "Sail is not executable: $sail_bin"
[ -d "$lean_sail_dir" ] || fail "missing Lean support: $lean_sail_dir"
[ -x "$lake_bin" ] || fail "Lake is not executable: $lake_bin"
actual_support_revision=$(git -C "$lean_sail_dir" rev-parse HEAD)
[ "$actual_support_revision" = "$lean_sail_revision" ] ||
  fail "Lean support revision is $actual_support_revision"
case "$($sail_bin --version)" in
  'Sail 0.20.2 '*) ;;
  *) fail 'Sail version is not 0.20.2' ;;
esac
case "$($lake_bin --version)" in
  *'Lean version 4.29.0'*) ;;
  *) fail 'Lean version is not 4.29.0' ;;
esac
readonly work_dir=$(mktemp -d /tmp/hyperray-addi.XXXXXX)
readonly output_dir="$work_dir/generated"
readonly source_fragment="$work_dir/official_addi.sail"
readonly harness="$work_dir/harness.sail"
mkdir -p "$output_dir"
"$script_dir/source.sh" "$source_dir" "$proof_dir/sail/prefix.sail" \
  "$source_fragment" "$harness"
"$sail_bin" --strict-var --strict-bitvector --strict-exponentials \
  --require-version 0.20.2 --lean --lean-output-dir "$output_dir" \
  --lean-force-output --lean-lib-path "$lean_sail_dir" \
  -o Official_ADDI_Slice "$harness" "$source_fragment"
readonly project_dir="$output_dir/Official_ADDI_Slice"
"$script_dir/measure.sh" "$project_dir" "$work_dir" "$lake_bin"
printf 'Integer proofs passed. All one-bit mutations failed as required.\n'
printf 'Proof records: %s\n' "$work_dir"
