#!/usr/bin/env bash
# Measure all generated integer proofs and their negative controls.
# Never accept a missing axiom report or counterexample.
set -euo pipefail

fail() {
  printf 'integer proof measurement error: %s\n' "$1" >&2
  exit 1
}

if [ "$#" -ne 3 ]; then
  fail 'usage: measure.sh PROJECT_DIR WORK_DIR LAKE_BIN'
fi

readonly project_dir=$1
readonly work_dir=$2
readonly lake_bin=$3
readonly script_dir=$(cd "$(dirname "$0")" && pwd)
readonly lean_dir=$(cd "$script_dir/../lean" && pwd)
readonly module_dir="$project_dir/OfficialADDISlice"
readonly object_dir="$project_dir/.lake/build/lib/lean/OfficialADDISlice"
readonly proof_log="$work_dir/proof.log"

run_proof() {
  (cd "$project_dir" && "$lake_bin" env lean \
    -o "$object_dir/$1.olean" \
    "OfficialADDISlice/$1.lean") 2>&1 | tee -a "$proof_log"
}

run_mutation() {
  local mutation=$1
  local mutation_log="$work_dir/$mutation.log"
  set +e
  (cd "$project_dir" && "$lake_bin" env lean \
    "OfficialADDISlice/$mutation.lean") 2>&1 | tee "$mutation_log"
  local mutation_status=${PIPESTATUS[0]}
  set -e
  [ "$mutation_status" -ne 0 ] || fail "the $mutation mutation passed"
  grep -q 'found a counterexample' "$mutation_log" ||
    fail "the $mutation mutation has no counterexample"
}

cp "$lean_dir/"{AddiProof,ITypeProof,UTypeDecode,UTypeProof}.lean "$module_dir/"
cp "$lean_dir/"{RTypeDecode,RTypeProof}.lean "$module_dir/"
cp "$lean_dir/"{ShiftIDecode,ShiftIProof,CatalogProof}.lean "$module_dir/"
cp "$lean_dir/"{AddiwDecode,AddiwProof}.lean "$module_dir/"
cp "$lean_dir/"{AddiMutation,UTypeMutation,RTypeMutation,ShiftIMutation}.lean \
  "$module_dir/"
cp "$lean_dir/AddiwMutation.lean" \
  "$module_dir/"
(cd "$project_dir" && "$lake_bin" update && "$lake_bin" build)
: > "$proof_log"
for proof in ITypeProof AddiProof UTypeDecode UTypeProof RTypeDecode \
  RTypeProof ShiftIDecode ShiftIProof CatalogProof AddiwDecode AddiwProof; do
  run_proof "$proof"
done
for theorem in officialITypeDecode officialExecuteITypeBridge \
  officialITypeCircuitEquivalence officialITypeCatalog officialUTypeDecode \
  officialExecuteUTypeBridge officialUTypeCircuitEquivalence officialUTypeCatalog \
  officialRTypeDecode officialExecuteRTypeBridge \
  officialRTypeCircuitEquivalence officialRTypeCatalog \
  officialShiftImmediateDecode officialExecuteShiftImmediateBridge \
  officialShiftImmediateCircuitEquivalence officialShiftImmediateCatalog \
  officialAddiwDecode officialExecuteAddiwBridge \
  officialAddiwCircuitEquivalence; do
  grep -q "'$theorem' depends on axioms" "$proof_log" ||
    fail "the $theorem axiom report is missing"
done
for mutation in AddiMutation UTypeMutation RTypeMutation ShiftIMutation \
  AddiwMutation; do
  run_mutation "$mutation"
done
