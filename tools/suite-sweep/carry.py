#!/usr/bin/env python3
# Carry forward a result only when the code that was proved is byte-identical.
# A verdict belongs to the machine code it ran against, never to a name or a
# position, so a renamed or relabelled case keeps its proof and an edited one
# is proved again.
import json
import pathlib
import sys


def compiled_code(entry: dict) -> tuple:
    return (entry.get("rust_expression", ""), entry.get("rust_expected", ""))


def carry(previous: list, cases: list) -> list:
    """Results whose compiled code still exists, renamed to the current case."""
    current = {compiled_code(case): case for case in cases}
    kept = []
    for entry in previous:
        case = current.get(compiled_code(entry))
        if case:
            kept.append({**case, "expected_value": entry.get("expected_value"),
                         "result": entry["result"]})
    return kept


def main() -> int:
    directory = pathlib.Path(sys.argv[1])
    previous = json.loads(pathlib.Path(sys.argv[2]).read_text())
    cases = json.loads((directory / "cases.json").read_text())
    kept = carry(previous, cases)
    (directory / "results.json").write_text(json.dumps(kept, indent=2))
    print(f"carried {len(kept)} of {len(previous)} results onto {len(cases)} cases")
    return 0


if __name__ == "__main__":
    sys.exit(main())
