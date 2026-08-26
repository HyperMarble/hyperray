#!/usr/bin/env bash
# Builds a pinned, patched touchstone-prover venv for ray's Layer 3 (oracle).
#
# Pins touchstone-prover==1.60.0 and applies ray-typed.patch: an unmodeled
# dependency call whose result is annotated with a declared scalar type
# (`x: bool = f(...)`) is narrowed to a free symbolic value of that type,
# instead of an opaque, uncomparable one -- without hand-writing a stub for
# every function of every dependency, and independent of touchstone's
# broader/riskier internal _TRAPFREE flag. See ray-typed.patch for the
# annotated diff and docs/specs for the reasoning.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV="${1:-$HERE/venv}"

python3 -m venv "$VENV"
"$VENV/bin/pip" install --quiet touchstone-prover==1.60.0

CORE="$("$VENV/bin/python3" -c 'import touchstone.core as c; print(c.__file__)')"
patch "$CORE" < "$HERE/ray-typed.patch"

"$VENV/bin/python3" "$HERE/oracle_driver.py" --self-check

echo "ray oracle venv ready: $VENV"
