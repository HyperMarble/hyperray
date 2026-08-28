#!/usr/bin/env python3
"""ray Layer 4 (diff-test): runs the real solution and the proven oracle
model on the same concrete inputs and reports where they disagree.

This is the layer that closes the gap between "we proved something" and
"the shipped code actually does it". Layer 3 proves a property of a
simplified reference model; nothing in that proof says the real
implementation matches the model. Running both on real inputs and
comparing is what catches the drift.

Disagreement is judged on observable behaviour, which is either a
returned value or a raised exception:

  * both return   -> agree iff the values are equal
  * both raise    -> agree iff the exception TYPES match; messages are
                     deliberately not compared, since spec.md states
                     required behaviour ("raise ValueError containing
                     ...") rather than exact wording, and two correct
                     implementations may word a message differently
  * one returns,
    one raises    -> disagree, always

Reads one JSON request on stdin:
  {"model_src": "...", "model_fn": "f",
   "real_src": "...",  "real_fn": "f",
   "inputs": [[arg, ...], ...]}

Writes one JSON response on stdout:
  {"total": N, "agreements": N, "disagreements": [
     {"input": [...], "model": {...}, "real": {...}}, ...]}
"""
import json
import sys


def _load(src, fn_name, namespace_name):
    """Execute one source string in its own namespace and return the
    named function. The two sides are kept in separate namespaces so a
    helper defined in one cannot satisfy a missing name in the other --
    that would mask a real difference."""
    ns = {"__name__": namespace_name}
    exec(compile(src, f"<{namespace_name}>", "exec"), ns)
    if fn_name not in ns:
        raise KeyError(f"{fn_name!r} not defined in {namespace_name}")
    return ns[fn_name]


def _observe(fn, args):
    """Run one side and record what is observable: a returned value, or
    the type of exception raised."""
    try:
        return {"outcome": "return", "value": fn(*args)}
    except Exception as exc:  # noqa: BLE001 - any exception is an outcome
        return {"outcome": "raise", "exception_type": type(exc).__name__,
                "message": str(exc)}


def _agree(a, b):
    if a["outcome"] != b["outcome"]:
        return False
    if a["outcome"] == "raise":
        return a["exception_type"] == b["exception_type"]
    try:
        return bool(a["value"] == b["value"])
    except Exception:  # noqa: BLE001 - an uncomparable pair is a difference
        return False


def _jsonable(value):
    """Values come back from arbitrary real code, so they are not always
    JSON-serializable; fall back to repr so a report is never lost."""
    try:
        json.dumps(value)
        return value
    except (TypeError, ValueError):
        return repr(value)


def run(req):
    model = _load(req["model_src"], req["model_fn"], "ray_model")
    real = _load(req["real_src"], req["real_fn"], "ray_real")

    disagreements = []
    agreements = 0
    # How many inputs made the model actually return a value. Agreement on
    # inputs that only ever raised is not evidence of equivalence -- it can
    # simply mean the inputs did not fit the function's signature.
    returned_normally = 0
    for args in req["inputs"]:
        m = _observe(model, args)
        r = _observe(real, args)
        if m["outcome"] == "return":
            returned_normally += 1
        if _agree(m, r):
            agreements += 1
            continue
        if "value" in m:
            m = dict(m, value=_jsonable(m["value"]))
        if "value" in r:
            r = dict(r, value=_jsonable(r["value"]))
        disagreements.append({"input": [_jsonable(a) for a in args],
                              "model": m, "real": r})

    return {"total": len(req["inputs"]),
            "agreements": agreements,
            "returned_normally": returned_normally,
            "disagreements": disagreements}


if __name__ == "__main__":
    print(json.dumps(run(json.load(sys.stdin))))
