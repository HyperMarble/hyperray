#!/bin/bash
# Prove every assertion in the Rust integer suite against real machine code.
#
#   ./sweep.sh            prove what is not yet proved
#   ./sweep.sh 20         stop after the first 20 assertions
#   ./sweep.sh --refresh  re-read the suite after a rust-lang update
#
# --refresh re-extracts the suite, keeps every result whose compiled code is
# unchanged, and proves only what upstream actually changed.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROGRAMS="${SWEEP_PROGRAMS:-/Volumes/Hak_SSD/hyperray-build/suite-sweep/programs}"
ENVIRONMENT="${SWEEP_ENV:-/Volumes/Hak_SSD/hyperray-build/leaky-relu-e2e-20260911/env.sh}"

if [ ! -f "$ENVIRONMENT" ]; then
    echo "missing tool environment: $ENVIRONMENT" >&2
    exit 1
fi
# shellcheck source=/dev/null
source "$ENVIRONMENT"

REFRESH=""
LIMIT=""
for argument in "$@"; do
    case "$argument" in
        --refresh) REFRESH=1 ;;
        *) LIMIT="$argument" ;;
    esac
done

mkdir -p "$PROGRAMS"
if [ -n "$REFRESH" ]; then
    KEPT="$PROGRAMS/results.json"
    PRIOR="$PROGRAMS/results-before-refresh.json"
    if [ -f "$KEPT" ]; then
        cp "$KEPT" "$PRIOR"
    fi
    python3 "$HERE/generate.py" "$PROGRAMS"
    if [ -f "$PRIOR" ]; then
        python3 "$HERE/carry.py" "$PROGRAMS" "$PRIOR"
    fi
fi

exec python3 "$HERE/sweep.py" "$PROGRAMS" ${LIMIT:+"$LIMIT"}
