#!/usr/bin/env python3
# A case is named by what it asserts, never by where it sits in the file.
# rust-lang edits int_macros.rs, and a positional name silently re-points an
# old result at new code: insert one assertion at the top and every later
# index shifts by one, so "s007 PROVED" would claim a proof never run.
import hashlib
import json
import pathlib


def identity(test: str, expression: str, expected: str) -> str:
    """The name of a case, derived from the code that is actually compiled.

    The suite reuses one assertion text in several blocks, each with a
    different constant, so the name must come from the resolved text.
    """
    return "t" + hashlib.sha256("\n".join((test, expression, expected)).encode()).hexdigest()[:12]


def compare(previous: list, current: list) -> dict:
    """Names what an upstream update added, removed, and left alone."""
    before = {entry["name"] for entry in previous}
    after = {entry["name"] for entry in current}
    return {
        "added": sorted(after - before),
        "removed": sorted(before - after),
        "unchanged": sorted(before & after),
    }


def load(path: pathlib.Path) -> list:
    if not path.exists():
        return []
    return json.loads(path.read_text())
