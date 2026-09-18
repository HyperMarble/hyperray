#!/usr/bin/env bash
# Replay upstream event identities and negative coverage claims.
# A passing audit never authorizes a whole-program proof verdict.
set -euo pipefail

fail() {
  printf 'event proof error: %s\n' "$1" >&2
  exit 1
}

[ "$#" -eq 2 ] || fail 'usage: replay.sh MODEL_DIR LAKE_BIN'
readonly model_dir=$1
readonly lake_bin=$2
readonly support_dir="$model_dir/.lake/packages/Sail"
readonly script_dir=$(cd "$(dirname "$0")" && pwd)
readonly proof_dir=$(cd "$script_dir/../lean" && pwd)
readonly fixture_dir=$(cd "$script_dir/../../../fixtures/lean" && pwd)
readonly model_revision=51c635c460125ac87955d061fcd902ce03b8011f
readonly support_revision=079463134b9c50450b8393e1566a09fc492a34d9

[ -x "$lake_bin" ] || fail "Lake is not executable: $lake_bin"
[ -d "$support_dir" ] || fail "missing Sail support: $support_dir"
[ "$(git -C "$model_dir" rev-parse HEAD)" = "$model_revision" ] ||
  fail 'generated model revision differs'
[ "$(git -C "$support_dir" rev-parse HEAD)" = "$support_revision" ] ||
  fail 'Sail support revision differs'
[ -z "$(git -C "$model_dir" status --porcelain)" ] ||
  fail 'generated model has local changes'
[ -z "$(git -C "$support_dir" status --porcelain)" ] ||
  fail 'Sail support has local changes'
case "$("$lake_bin" --version)" in
  *'Lean version 4.29.0'*) ;;
  *) fail 'Lean release differs from 4.29.0' ;;
esac

cd "$model_dir"
for name in Preservation RangeMismatch; do
  printf 'Event proof: %s\n' "$name"
  "$lake_bin" env lean -DwarningAsError=true "$proof_dir/$name.lean"
done
for name in changed-barrier changed-range; do
  if output=$("$lake_bin" env lean -DwarningAsError=true "$fixture_dir/$name.lean" 2>&1); then
    fail "false claim accepted: $name"
  fi
  case "$output" in
    *'error: Tactic `rfl` failed:'*) ;;
    *)
      printf '%s\n' "$output" >&2
      fail "unexpected rejection: $name"
      ;;
  esac
  printf 'False claim rejected: %s\n' "$name"
done
printf 'Imported event audit passed; full coverage remains open.\n'
