#!/usr/bin/env bash
# Build proof input from pinned official upper, immediate, and register source.
# Reject a changed source before any proof starts.
set -euo pipefail

fail() {
  printf 'integer source error: %s\n' "$1" >&2
  exit 1
}

if [ "$#" -ne 4 ]; then
  fail 'usage: source.sh SAIL_RISCV_DIR PREFIX SOURCE_OUTPUT HARNESS_OUTPUT'
fi

readonly source_dir=$1
readonly prefix=$2
readonly source_output=$3
readonly harness_output=$4
readonly revision=abeec0f2eb20b5508b756c37e7274a7e5919ac15
readonly types_path=model/extensions/I/base_types.sail
readonly instructions_path=model/extensions/I/base_insts.sail
readonly prelude_path=model/prelude/prelude.sail
readonly types_file="$source_dir/$types_path"
readonly instructions_file="$source_dir/$instructions_path"
readonly prelude_file="$source_dir/$prelude_path"

[ -f "$types_file" ] || fail "missing source file: $types_file"
[ -f "$instructions_file" ] || fail "missing source file: $instructions_file"
[ -f "$prelude_file" ] || fail "missing source file: $prelude_file"
[ -f "$prefix" ] || fail "missing harness prefix: $prefix"

actual_revision=$(git -C "$source_dir" rev-parse HEAD)
[ "$actual_revision" = "$revision" ] ||
  fail "source revision is $actual_revision"
git -C "$source_dir" diff --quiet -- \
  "$types_path" "$instructions_path" "$prelude_path" ||
  fail 'the integer source has local changes'
git -C "$source_dir" diff --cached --quiet -- \
  "$types_path" "$instructions_path" "$prelude_path" ||
  fail 'the integer source has staged changes'

sed -n -e '37,41p' -e '177,188p' "$prelude_file" > "$source_output"
sed -n '16,34p' "$instructions_file" >> "$source_output"
sed -n '143,169p' "$instructions_file" >> "$source_output"
sed -n '184,211p' "$instructions_file" >> "$source_output"
sed -n '223,251p' "$instructions_file" >> "$source_output"
sed -n '337,348p' "$instructions_file" >> "$source_output"
sed -n '355,386p' "$instructions_file" >> "$source_output"
sed -n '401,423p' "$instructions_file" >> "$source_output"
sed -n '1,7p' "$prefix" > "$harness_output"
sed -n '9p' "$types_file" >> "$harness_output"
sed -n '11p' "$types_file" >> "$harness_output"
sed -n '12p' "$types_file" >> "$harness_output"
sed -n '13,14p' "$types_file" >> "$harness_output"
sed -n '16,17p' "$types_file" >> "$harness_output"
sed -n '8,$p' "$prefix" >> "$harness_output"
