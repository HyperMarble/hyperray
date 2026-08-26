#!/usr/bin/env python3
"""ray Layer 3 (oracle) driver: reads one proof request as JSON on stdin,
calls the patched touchstone-prover, writes one verdict as JSON on stdout.

Request:  {"src": "<function source>", "ensures": "<postcondition>",
           "requires": "<precondition>" (optional, default "True"),
           "best_effort": <bool> (optional, default true)}
Response: {"status": "PROVED"|"REFUTED"|"UNKNOWN", "reason": "...",
           "counterexample": "..." or null}

Kept deliberately thin: all real reasoning is touchstone's (patched) and
ray's own -- this file only bridges JSON in/out so internal/oracle can
shell out to it the same way internal/coverage shells out to pict.
"""
import json
import sys

from touchstone import core, engines


def run(req):
    core.BEST_EFFORT = bool(req.get("best_effort", True))
    requires = req.get("requires", "True")
    v = engines.prove(req["src"], ensures=req["ensures"], requires=requires)
    return {
        "status": v.status,
        "reason": v.reason,
        "counterexample": v.counterexample,
    }


def self_check():
    """Sanity check run by build.sh right after applying the patch: confirms
    the venv is wired correctly (patched module importable, ordinary proving
    still works, the ray-typed annotation narrowing still fires) before
    trusting the build."""
    assert core._TRAPFREE is False, "self-check must not depend on _TRAPFREE"

    plain = run({
        "src": "def clamp(x: int, lo: int, hi: int):\n"
               "    if x < lo:\n        return lo\n"
               "    if x > hi:\n        return hi\n    return x\n",
        "ensures": "result >= lo and result <= hi",
        "requires": "lo <= hi",
    })
    assert plain["status"] == "PROVED", f"plain proving broken: {plain}"

    typed = run({
        "src": "def check(instance):\n"
               "    import jsonschema\n"
               "    schema = {'type': 'string'}\n"
               "    ok: bool = jsonschema.Draft7Validator(schema).is_valid(instance)\n"
               "    return ok\n",
        "ensures": "result == True or result == False",
    })
    assert typed["status"] == "PROVED", f"ray-typed patch not active: {typed}"

    print("self-check passed: plain proving + ray-typed narrowing both work", file=sys.stderr)


if __name__ == "__main__":
    if "--self-check" in sys.argv:
        self_check()
        sys.exit(0)
    request = json.load(sys.stdin)
    print(json.dumps(run(request)))
