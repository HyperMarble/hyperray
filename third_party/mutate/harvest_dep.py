#!/usr/bin/env python3
"""Harvest a pinned dependency's own tested edge values.

An SMT solver cannot reason through a complex dependency's internals, so ray
does not try: the dependency's own test suite is a curated list of inputs its
authors proved they care about. Harvesting the literals from those tests
yields solver-grade edge values without symbolically entering the code.

Output: JSON {package, version, values: {callee: [literal, ...]}} on stdout.
Only literals at call sites in the package's own test files are collected,
so every value comes with the implicit endorsement of an upstream test.
"""
import ast
import importlib.metadata
import importlib.util
import json
import pathlib
import sys

EDGE_HINTS = {0, 1, -1, 2, "", " ", None, True, False}


def edge_rank(value):
    """Lower ranks first: canonical edges, then short values, then the rest."""
    if isinstance(value, float) and (value != value or value in (float("inf"), float("-inf"))):
        return 0
    try:
        if value in EDGE_HINTS:
            return 0
    except TypeError:
        pass
    if isinstance(value, (list, tuple, dict)) and len(value) == 0:
        return 0
    return 1 if len(repr(value)) <= 8 else 2


def literal(node):
    try:
        return True, ast.literal_eval(node)
    except (ValueError, SyntaxError):
        return False, None


def harvest(package):
    spec = importlib.util.find_spec(package)
    if spec is None or not spec.submodule_search_locations:
        raise SystemExit(f"package {package!r} is not installed in this interpreter")
    root = pathlib.Path(list(spec.submodule_search_locations)[0])
    values = {}
    for path in sorted(root.rglob("test_*.py")) + sorted(root.rglob("*_test.py")):
        try:
            tree = ast.parse(path.read_text(errors="replace"))
        except SyntaxError:
            continue
        for node in ast.walk(tree):
            if not isinstance(node, ast.Call):
                continue
            callee = node.func
            name = callee.attr if isinstance(callee, ast.Attribute) else getattr(callee, "id", None)
            if not name or name.startswith("_"):
                continue
            for argument in list(node.args) + [kw.value for kw in node.keywords]:
                ok, value = literal(argument)
                if not ok:
                    continue
                bucket = values.setdefault(name, [])
                rendered = repr(value)
                if rendered not in {repr(v) for v in bucket}:
                    bucket.append(value)
    for name in values:
        values[name].sort(key=lambda v: (edge_rank(v), repr(v)))
        values[name] = values[name][:24]
    return {name: bucket for name, bucket in values.items() if bucket}


def main():
    package = sys.argv[1]
    try:
        version = importlib.metadata.version(package)
    except importlib.metadata.PackageNotFoundError:
        version = "unknown"
    json.dump(
        {"package": package, "version": version, "values": harvest(package)},
        sys.stdout, indent=1, default=repr,
    )
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
