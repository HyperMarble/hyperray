#!/usr/bin/env python3
# Prove every extracted assertion and record the engine's own outcome.
# A blocked case keeps the reason the engine reported, never a substitute.
import json
import pathlib
import re
import sys

from build import build, run
from binary_facts import entry_and_end
from proof_request import HYPERRAY, expected_value, request


def verdict(text: str) -> str:
    status = re.search(r'"status": "([A-Z]+)"', text)
    if status:
        return status.group(1)
    reason = re.search(r'err: ([A-Za-z]+)\(?"?([^",)]{0,48})', text)
    if reason:
        return "BLOCKED: " + (reason.group(1) + " " + reason.group(2)).strip()
    first = text.strip().split("\n")[0]
    return "BLOCKED: " + first[:60]

def main() -> int:
    directory = pathlib.Path(sys.argv[1])
    cases = json.loads((directory / "cases.json").read_text())
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else len(cases)
    results = []
    for case in cases[:limit]:
        name = case["name"]
        failure = build(directory, name)
        if failure:
            results.append({**case, "result": "BLOCKED: " + failure})
            continue
        binary = directory / f"{name}.bin"
        start, end = entry_and_end(binary)
        if end is None:
            results.append({**case, "result": "BLOCKED: no return instruction found"})
            continue
        value, failure = expected_value(directory, name, case["rust_expected"])
        if failure:
            results.append({**case, "result": "BLOCKED: " + failure})
            continue
        (directory / f"{name}-req.json").write_text(
            json.dumps(request(binary, name, start, end, value), indent=2)
        )
        output = run([HYPERRAY, "machine", str(directory / f"{name}-req.json")])
        results.append({**case, "expected_value": value,
                        "result": verdict(output.stdout + output.stderr)})
        print(f"{name} {case['test']:28s} {results[-1]['result']}", flush=True)
    (directory / "results.json").write_text(json.dumps(results, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
