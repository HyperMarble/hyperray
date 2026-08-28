#!/usr/bin/env python3
"""Maps every source line to the tests that execute it.

This is the optimisation that makes obligation B affordable. Without it,
each adversary costs one run of the whole verifier -- 41 seconds on a real
Shipd task, so a hundred adversaries is over an hour. With it:

  * a line NO test executes needs no test run at all. An adversary there
    cannot be caught, so if it deviates it is a false positive by
    construction, found in zero seconds.
  * a line some tests execute needs only THOSE tests, not the suite. Two
    or three tests instead of twenty-four.

Uses coverage.py's dynamic contexts, which record which test was running
when each line executed. That is a measurement, not a guess: nothing here
infers which test "probably" covers a line.

Usage:
    line_tests.py --root DIR --tests PATH [--source PKG] [--out FILE]

Writes JSON: {"relative/file.py": {"137": ["test_a", "test_b"], ...}}
"""
import argparse
import json
import os
import subprocess
import sys
import tempfile


def run_with_contexts(root, tests, source):
    """Run the suite once, recording which test executed each line."""
    data_file = tempfile.mktemp(suffix=".coverage")
    cmd = [
        sys.executable, "-m", "pytest", "-q", "-p", "no:cacheprovider",
        f"--cov={source}", "--cov-context=test", "--cov-report=",
        tests,
    ]
    env = {**os.environ, "COVERAGE_FILE": data_file, "PYTHONDONTWRITEBYTECODE": "1"}
    proc = subprocess.run(cmd, cwd=root, env=env, capture_output=True, text=True)
    return data_file, proc


def contexts_by_line(data_file):
    """Read the recorded contexts back out of coverage's own database."""
    import coverage

    cov = coverage.Coverage(data_file=data_file)
    cov.load()
    data = cov.get_data()

    mapping = {}
    for measured in data.measured_files():
        per_line = data.contexts_by_lineno(measured)
        if not per_line:
            continue
        lines = {}
        for lineno, contexts in per_line.items():
            names = set()
            for ctx in contexts:
                if not ctx:
                    continue
                # coverage records "path::test_name|phase"; the test's own
                # name is what a task's one_test command takes.
                name = ctx.split("|")[0].split("::")[-1]
                if name:
                    names.add(name)
            if names:
                lines[str(lineno)] = sorted(names)
        if lines:
            mapping[measured] = lines
    return mapping


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True, help="directory to run the suite from")
    ap.add_argument("--tests", required=True, help="test path passed to pytest")
    ap.add_argument("--source", default=".", help="package or path to measure")
    ap.add_argument("--out", default="-")
    args = ap.parse_args()

    data_file, proc = run_with_contexts(args.root, args.tests, args.source)
    try:
        mapping = contexts_by_line(data_file)
    except Exception as exc:  # noqa: BLE001 - a failed measurement must be loud
        print(json.dumps({"error": str(exc), "stderr": proc.stderr[-2000:]}),
              file=sys.stderr)
        return 2
    finally:
        for suffix in ("", ".lock"):
            try:
                os.remove(data_file + suffix)
            except OSError:
                pass

    text = json.dumps(mapping)
    if args.out == "-":
        print(text)
    else:
        with open(args.out, "w") as f:
            f.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
