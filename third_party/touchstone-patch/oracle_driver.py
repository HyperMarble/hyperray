#!/usr/bin/env python3
"""hyperray Layer 3 (oracle) driver: reads one proof request as JSON on stdin,
calls the patched touchstone-prover, writes one verdict as JSON on stdout.

Request:  {"src": "<function source>", "ensures": "<postcondition>",
           "requires": "<precondition>" (optional, default "True"),
           "best_effort": <bool> (optional, default true),
           "auto_annotate": <bool> (optional, default true)}
Response: {"status": "PROVED"|"REFUTED"|"UNKNOWN", "reason": "...",
           "counterexample": "..." or null}

Kept deliberately thin: all real reasoning is touchstone's (patched) and
hyperray's own -- this file only bridges JSON in/out so internal/oracle can
shell out to it the same way internal/coverage shells out to pict.

auto_annotate (on by default) runs auto_annotate.auto_annotate() on src
first: mypy resolves an unmodeled call's real declared return type from
installed PEP 561 stubs and adds the `: bool`/`int`/`float` annotation
automatically wherever it can. Where it can't -- an incomplete stub, no
stub installed at all -- the source passes through unchanged, same as
if auto_annotate were off; a model author's own manual annotation
(what hyperray-typed.patch itself consumes) is the fallback for those cases,
not something this driver invents or guesses at.
"""
import json
import sys

from touchstone import core, engines

import auto_annotate as _auto_annotate


def run(req):
    core.BEST_EFFORT = bool(req.get("best_effort", True))
    requires = req.get("requires", "True")
    src = req["src"]
    if req.get("auto_annotate", True):
        src = _auto_annotate.auto_annotate(src)
    v = engines.prove(src, ensures=req["ensures"], requires=requires)
    return {
        "status": v.status,
        "reason": v.reason,
        "counterexample": v.counterexample,
    }


def self_check():
    """Sanity check run by build.sh right after applying the patch: confirms
    the venv is wired correctly (patched module importable, ordinary proving
    still works, the hyperray-typed annotation narrowing still fires) before
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
    assert typed["status"] == "PROVED", f"hyperray-typed patch not active: {typed}"

    auto = run({
        "src": "def f(x):\n"
               "    import math\n"
               "    r = math.isnan(x)\n"
               "    return r\n",
        "ensures": "result == True or result == False",
    })
    assert auto["status"] == "PROVED", f"auto_annotate pre-pass not resolving math.isnan: {auto}"

    print("self-check passed: plain proving + hyperray-typed narrowing + auto_annotate all work", file=sys.stderr)


if __name__ == "__main__":
    if "--self-check" in sys.argv:
        self_check()
        sys.exit(0)
    request = json.load(sys.stdin)
    print(json.dumps(run(request)))
