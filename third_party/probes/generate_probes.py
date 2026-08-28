#!/usr/bin/env python3
"""Generates the inputs an adversary and the real solution are both run on.

Hand-writing them does not scale and does not generalise: measured on the
cron fixture, four hand-picked probes found ZERO false positives while ten
found nine, and the difference was entirely which inputs happened to be
chosen.

Hypothesis removes the choosing. The task declares the SHAPE of an input
once -- "five space-separated cron fields", "a diagnostics file" -- and
Hypothesis produces many concrete ones, biased toward the awkward values
that expose behaviour: empty, zero, boundaries, reversed ranges.

WHY NOT A SOLVER. CrossHair's `diffbehavior` decides this symbolically and
gives an exact witness, which is strictly better -- on a plain typed
function. Measured on this corpus it is not usable: `parse_cron` takes a
STRING, and pushing a symbolic string through split/partition/int is where
symbolic execution stalls. It returned "unknown" after 183 iterations on
the very adversary probes catch immediately. Every real task here is
string- or object-shaped, so probes are what work.

Reads a strategy expression on argv[1], prints one generated input per
line as JSON. The expression is evaluated with hypothesis.strategies in
scope as `st`, so a task writes a normal strategy and nothing here needs
to know the task's domain.

Values a caller wants guaranteed can be passed with --must-include; they
are emitted first. That is how boundary values the code itself names (a
constant an adversary shifts from 59 to 60 is the code saying 59 matters)
get into the probe set alongside the generated ones.
"""
import argparse
import json
import sys

from hypothesis import HealthCheck, given, settings
from hypothesis import strategies as st  # noqa: F401  (in scope for eval)


def generate(expression, count, seed):
    """Draw `count` distinct examples from the strategy `expression`."""
    strategy = eval(expression, {"st": st, "__builtins__": {}})  # noqa: S307

    seen = []

    @settings(max_examples=count * 20, database=None, deadline=None,
              suppress_health_check=list(HealthCheck), derandomize=True)
    @given(strategy)
    def collect(value):
        # Distinct inputs only: two identical probes cost a full verifier
        # run each and can never disagree with one another.
        if value not in seen and len(seen) < count:
            seen.append(value)

    collect()
    return seen


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("expression", help="a hypothesis strategy, with `st` in scope")
    ap.add_argument("--count", type=int, default=25)
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--must-include", action="append", default=[],
                    help="an input to emit first, regardless of what is generated")
    args = ap.parse_args()

    out = list(args.must_include)
    try:
        for value in generate(args.expression, args.count, args.seed):
            if value not in out:
                out.append(value)
    except Exception as exc:  # noqa: BLE001 - a bad strategy must not be silent
        print(json.dumps({"error": str(exc)}), file=sys.stderr)
        return 2

    for value in out:
        print(json.dumps(value))
    return 0


if __name__ == "__main__":
    sys.exit(main())
