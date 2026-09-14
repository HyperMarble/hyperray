#!/usr/bin/env python3
# Prove every extracted assertion and record the engine's own outcome.
# A blocked case keeps the reason the engine reported, never a substitute.
import json
import pathlib
import re
import sys

from build import build, run
from binary_facts import entry_and_end
from expected import expected_value
from proof_request import HYPERRAY, request
from progress import Progress


def verdict(text: str) -> str:
    status = re.search(r'"status": "([A-Z]+)"', text)
    if status:
        return status.group(1)
    reason = re.search(r'err: ([A-Za-z]+)\(?"?([^",)]{0,48})', text)
    if reason:
        return "BLOCKED: " + (reason.group(1) + " " + reason.group(2)).strip()
    first = text.strip().split("\n")[0]
    return "BLOCKED: " + first[:60]


def prove_one(directory, case: dict, report, position: int) -> dict:
    """Each stage is named before it runs, so a slow case shows where it is."""
    name = case["name"]
    report.begin(position, name, case["test"])
    report.stage("compiling")
    failure = build(directory, name)
    if failure:
        return {**case, "result": "BLOCKED: " + failure}
    report.stage("reading the binary")
    binary = directory / f"{name}.bin"
    start, end = entry_and_end(binary)
    if end is None:
        return {**case, "result": "BLOCKED: no return instruction found"}
    report.stage("evaluating expected")
    value, failure = expected_value(directory, name, case["rust_expected"])
    if failure:
        return {**case, "result": "BLOCKED: " + failure}
    (directory / f"{name}-req.json").write_text(
        json.dumps(request(binary, name, start, end, value), indent=2)
    )
    report.stage("proving")
    output = run([HYPERRAY, "machine", str(directory / f"{name}-req.json")])
    return {**case, "expected_value": value, "result": verdict(output.stdout + output.stderr)}


def main() -> int:
    directory = pathlib.Path(sys.argv[1])
    cases = json.loads((directory / "cases.json").read_text())
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else len(cases)
    finished = directory / "results.json"
    results = json.loads(finished.read_text()) if finished.exists() else []
    done = {entry["name"] for entry in results}
    report = Progress(min(limit, len(cases)), len(results))
    for position, case in enumerate(cases[:limit], start=1):
        if case["name"] in done:
            continue
        outcome = prove_one(directory, case, report, position)
        results.append(outcome)
        report.finish(outcome["result"])
        (directory / "results.json").write_text(json.dumps(results, indent=2))
    report.summary(results)
    (directory / "results.json").write_text(json.dumps(results, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
