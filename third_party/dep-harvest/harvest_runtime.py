#!/usr/bin/env python3
"""Harvests real edge-case values from a pinned dependency by RUNNING its
own test suite and recording the values that actually flow through it.

Why runtime and not static extraction: a dependency's maintainers already
found its edge cases and encoded them in its tests. Statically scraping
literals out of those test files only sees hardcoded constants -- it
misses fixtures, parametrized cases, computed setup, and anything built
at runtime, which is most of what a mature test suite actually exercises.
Executing the real suite against the real pinned version observes the
values themselves.

The version is whatever is installed in the interpreter running this
script, so pinning is inherited from the environment (a task's own
Dockerfile, or a locked venv) rather than re-declared here. The exact
version observed is reported in the output so a harvest is attributable.

Output JSON on stdout:
  {"module": "...", "version": "...", "tests": "...",
   "calls_observed": N,
   "values": {"int": [...], "float": [...], "str": [...],
              "bool": [...], "none": [...]}}
"""
import argparse
import json
import sys
from collections import defaultdict

# Bounds on how much is kept, so tracing a large suite cannot blow up
# memory or output size. These are resource limits, not judgements about
# which values are interesting.
_MAX_PER_KIND = 200
_MAX_SERIALIZED_LEN = 400


def _reusable(value):
    """Whether a value can actually be fed back in as a concrete test
    input by `coverage` and `difftest`.

    The criterion is a single question -- does it serialize? -- rather
    than a hand-listed set of accepted types. That one rule replaces what
    was previously five separate hand-written rules (a primitives tuple,
    a type-name mapping, a string-length cap, a dunder-name filter, and a
    NaN/inf special case), and it is also strictly better: lists and
    dicts of primitives now survive, where the type-tuple version
    discarded them outright even though `[]`, `{}` and nested structures
    are among the most valuable edge-case inputs a test suite produces.

    A live object fails to serialize and is correctly excluded, because
    it is not reusable as an input anyway.

    allow_nan=False is deliberate and load-bearing. Python's default
    emits bare `NaN`/`Infinity`, which are NOT valid JSON -- the Go side
    rejects them outright, which is exactly how this was caught. Strict
    mode makes the rule mean what it must mean here: "can this value
    round-trip as valid JSON", which is the real requirement for
    anything crossing into `coverage` and `difftest`. NaN is a genuine
    edge case, but it is not transportable through this pipeline in
    either direction, so excluding it is correct rather than a
    compromise."""
    try:
        encoded = json.dumps(value, allow_nan=False)
    except (TypeError, ValueError):
        return None
    if len(encoded) > _MAX_SERIALIZED_LEN:
        return None
    return encoded


def _harvest(module_prefix, test_path, max_calls):
    seen = defaultdict(set)
    stats = {"calls": 0}

    def tracer(frame, event, arg):
        if event != "call":
            return None
        if stats["calls"] >= max_calls:
            return None
        name = frame.f_globals.get("__name__", "")
        # Record values entering the dependency's own code, not its tests.
        if not name.startswith(module_prefix) or ".tests" in name or ".test_" in name:
            return None
        stats["calls"] += 1
        code = frame.f_code
        for varname in code.co_varnames[: code.co_argcount]:
            encoded = _reusable(frame.f_locals.get(varname))
            if encoded is None:
                continue
            # Group by Python's own type name rather than a mapping of
            # ours -- the language already answers "what kind is this".
            kind = type(frame.f_locals.get(varname)).__name__
            if len(seen[kind]) < _MAX_PER_KIND:
                seen[kind].add(encoded)
        return None

    import contextlib

    import pytest

    sys.settrace(tracer)
    try:
        # pytest writes its progress report to stdout, which would
        # corrupt this script's JSON output. Send it to stderr so stdout
        # carries the harvest and nothing else.
        with contextlib.redirect_stdout(sys.stderr):
            # Deliberately NOT -x: a failing or uncollectable test in the
            # dependency's own suite must not stop the harvest, because
            # the values observed along every other path are still real.
            # -p no:cacheprovider keeps the installed tree unmodified.
            pytest.main([test_path, "-q", "--no-header", "--tb=no",
                         "-p", "no:cacheprovider",
                         "--continue-on-collection-errors"])
    except SystemExit:
        pass
    finally:
        sys.settrace(None)

    # Values are stored already-serialized, so they are decoded back once
    # here; sorting is on the encoded form, which is always comparable.
    return ({kind: [json.loads(v) for v in sorted(values)]
             for kind, values in seen.items()},
            stats["calls"])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--module", required=True,
                    help="dependency module prefix to observe, e.g. jsonschema")
    ap.add_argument("--tests", required=True,
                    help="path to that dependency's own test suite")
    ap.add_argument("--max-calls", type=int, default=200000)
    args = ap.parse_args()

    # importlib.metadata is the supported way to read an installed
    # version; module.__version__ is deprecated in some packages and
    # absent in others.
    try:
        from importlib.metadata import version as _dist_version
        version = _dist_version(args.module)
    except Exception:
        version = "unknown"

    values, calls = _harvest(args.module, args.tests, args.max_calls)
    json.dump({
        "module": args.module,
        "version": version,
        "tests": args.tests,
        "calls_observed": calls,
        "values": values,
    }, sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
