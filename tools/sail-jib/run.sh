#!/usr/bin/env bash
# Generate and validate the JIB constructor census for pinned RV64 base-I.
# Emit no completed report after a source, tool, or lowering error.
set -euo pipefail

fail() {
  printf 'Sail JIB error: %s\n' "$1" >&2
  exit 1
}

[ "$#" -eq 2 ] || fail 'usage: run.sh SAIL_RISCV_DIR SAIL_BIN'
readonly source_dir=$1
readonly sail_bin=$2
readonly script_dir=$(cd "$(dirname "$0")" && pwd)
readonly root_dir=$(cd "$script_dir/../.." && pwd)
readonly tool_bin_dir=$(cd "$(dirname "$sail_bin")" && pwd)
readonly tool_prefix=$(cd "$tool_bin_dir/.." && pwd)
readonly dune_bin="$tool_bin_dir/dune"
readonly sail_source="$tool_prefix/.opam-switch/sources/sail.0.20.2"
readonly backend="$sail_source/src/sail_c_backend/c_backend.ml"
readonly keywords="$sail_source/src/sail_c_backend/keywords.ml"
readonly revision=abeec0f2eb20b5508b756c37e7274a7e5919ac15
readonly backend_digest=e99e5e78ab69cc97220b6b047c1e78bd83639e02919c37cc7567754b3e17f81e
readonly keywords_digest=00e0957ef50c1c02b785b6a663a330f528ed476fa70eb163aba71274e3dff2fb
readonly config="$source_dir/build/config/rv64d_v256_e32.json"
readonly config_digest=ebd1ca3de6444ee673cc4a4e2d30a978edb2dec02b9da1768495c6e10b48daf0

[ -x "$sail_bin" ] || fail "Sail is not executable: $sail_bin"
[ -x "$dune_bin" ] || fail "Dune is not executable: $dune_bin"
[ -f "$backend" ] || fail "missing Sail C backend source: $backend"
[ -f "$keywords" ] || fail "missing Sail C backend keywords: $keywords"
[ -f "$config" ] || fail "missing configuration: $config"
[ "$(git -C "$source_dir" rev-parse HEAD)" = "$revision" ] || fail 'wrong Sail RISC-V revision'
git -C "$source_dir" diff --quiet || fail 'the Sail RISC-V source has local changes'
git -C "$source_dir" diff --cached --quiet || fail 'the Sail RISC-V source has staged changes'
[ "$(shasum -a 256 "$backend" | awk '{print $1}')" = "$backend_digest" ] || fail 'wrong Sail C backend source'
[ "$(shasum -a 256 "$keywords" | awk '{print $1}')" = "$keywords_digest" ] || fail 'wrong Sail C backend keywords'
[ "$(shasum -a 256 "$config" | awk '{print $1}')" = "$config_digest" ] || fail 'wrong configuration'
case "$($sail_bin --version)" in
  'Sail 0.20.2 '*) ;;
  *) fail 'Sail version is not 0.20.2' ;;
esac

readonly work_dir=$(mktemp -d /tmp/hyperray-sail-jib.XXXXXX)
readonly plugin_dir="$work_dir/plugin"
mkdir "$plugin_dir"
cp "$script_dir"/ocaml/* "$plugin_dir/"
cp "$backend" "$plugin_dir/c_backend.ml"
cp "$keywords" "$plugin_dir/keywords.ml"
env PATH="$tool_bin_dir:$PATH" OCAMLPATH="$tool_prefix/lib" \
  "$dune_bin" build --root "$plugin_dir" jib_plugin.cmxs
readonly plugin="$plugin_dir/_build/default/jib_plugin.cmxs"
readonly output="$work_dir/rv64-base-i"
(cd "$source_dir/model" && "$sail_bin" \
  --plugin "$plugin" --strict-var --strict-bitvector --strict-exponentials \
  --require-version 0.20.2 --memo-z3-path "$work_dir/jib.memo" \
  --config "$config" --jibcatalog -o "$output" \
  I_insts postlude riscv.sail_project) 2>&1 | tee "$work_dir/sail.log"
(cd "$root_dir" && go run ./tools/sail-jib/go "$output.json") | tee "$work_dir/validation.log"
printf 'JIB catalog records: %s\n' "$work_dir"
