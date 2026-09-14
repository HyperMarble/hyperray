#!/usr/bin/env python3
# The suite gives a constant its value inside a block, and reuses the same
# name in later blocks: R is defined 9 times as 0, 1, -1, 2, -2, 4, -4, MAX,
# and MIN. A flat table keeps only the last one, so 113 of 188 assertions
# would be compiled against the wrong value. Scope is tracked instead.
import re

# Constants are declared as $T, u32, and $U. Matching only $T missed 10 of
# the 54 declarations, leaving HALF and the SHIFT_AMOUNT_* names unresolved.
CONSTANT = re.compile(r"const (\w+)\s*:\s*([^=]+?)\s*=\s*([^;]+);")


def literal(value: str, declared: str) -> str:
    """The suite's constant value, written with the type the suite declared.

    A shift amount is u32, so it must not be given an i64 suffix. The value
    keeps the suite's own expression; only $T is resolved to the tested type.
    """
    text = value.strip().replace("<$T>", "i64").replace("$T", "i64").replace("$U", "u64")
    if declared.strip() in ("$T", "$U"):
        suffix = "i64" if declared.strip() == "$T" else "u64"
        return f"(({text}) as {suffix})"
    return f"(({text}) as {declared.strip()})"


def resolve(scopes: list, name: str) -> str:
    """The value of a name in the innermost block that defines it."""
    for scope in reversed(scopes):
        if name in scope:
            return scope[name]
    return ""


def scoped_constants(source: str) -> list:
    """One constant table per line, holding what is visible at that line."""
    scopes = [{}]
    visible = []
    for line in source.split("\n"):
        found = CONSTANT.search(line)
        if found:
            scopes[-1][found.group(1)] = literal(found.group(3), found.group(2))
        visible.append({name: resolve(scopes, name)
                        for scope in scopes for name in scope})
        depth = line.count("{") - line.count("}")
        scopes = open_and_close(scopes, depth)
    return visible


def open_and_close(scopes: list, depth: int) -> list:
    """Blocks the line opened become new tables; blocks it closed are dropped."""
    for _ in range(max(depth, 0)):
        scopes = scopes + [{}]
    for _ in range(max(-depth, 0)):
        scopes = scopes[:-1] if len(scopes) > 1 else scopes
    return scopes
