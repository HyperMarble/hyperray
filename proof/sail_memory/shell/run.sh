#!/usr/bin/env bash
# Replay imported memory proofs against exact upstream revisions.
# A successful replay does not establish whole-machine coverage.
set -euo pipefail

fail() {
  printf 'memory proof error: %s\n' "$1" >&2
  exit 1
}

if [ "$#" -ne 2 ]; then
  fail 'usage: run.sh MODEL_DIR LAKE_BIN'
fi

readonly model_dir=$1
readonly lake_bin=$2
readonly model_revision=51c635c460125ac87955d061fcd902ce03b8011f
readonly support_revision=079463134b9c50450b8393e1566a09fc492a34d9
readonly script_dir=$(cd "$(dirname "$0")" && pwd)
readonly proof_dir=$(cd "$script_dir/../lean" && pwd)
readonly support_dir="$model_dir/.lake/packages/Sail"
readonly mutation="$script_dir/../../../fixtures/lean/changed-read.lean"

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
case "$($lake_bin --version)" in
  *'Lean version 4.29.0'*) ;;
  *) fail 'Lean release differs from 4.29.0' ;;
esac

proofs=(AbsentRead DistinctAddress ReadAfterWrite)
cd "$model_dir"
for name in "${proofs[@]}"; do
  proof="$proof_dir/$name.lean"
  [ -f "$proof" ] || fail "missing required proof: $name"
  printf 'Memory proof: %s\n' "$proof"
  "$lake_bin" env lean -DwarningAsError=true "$proof"
done
if mutation_output=$("$lake_bin" env lean -DwarningAsError=true "$mutation" 2>&1); then
  fail 'changed read-after-write claim was accepted'
fi
case "$mutation_output" in
  *'error: Tactic `rfl` failed:'*) ;;
  *)
    printf '%s\n' "$mutation_output" >&2
    fail 'mutation failed for an unexpected reason'
    ;;
esac
printf 'Changed memory claim rejected.\n'
printf 'Imported memory proof replay passed.\n'
