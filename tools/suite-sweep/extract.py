#!/usr/bin/env python3
# Extract assertions from the Rust integer test suite and prove each one
# against real machine code. The assertion text is Rust's, never ours.
import json
import pathlib
import re
import sys

LIBRARY = pathlib.Path(
    "/Users/hak/.rustup/toolchains/stable-aarch64-apple-darwin/lib/rustlib/src/rust/library"
)
SUITE = LIBRARY / "coretests/tests/num/int_macros.rs"

# The suite instantiates its macro for each integer type. These are its own
# constants, read from the same file rather than restated here.
CONSTANTS = {}


def read_constants(source: str) -> dict:
    found = {}
    for name, value in re.findall(r"const (\w+): \$T = ([^;]+);", source):
        text = value.strip()
        if text in ("$T::MAX", "$T::MIN"):
            found[name] = "i64::" + text.split("::")[1]
        elif text == "!0":
            found[name] = "!0i64"
        else:
            found[name] = text + "i64"
    return found


def extract(source: str) -> list:
    """One entry per assertion the suite states about a computed value."""
    cases = []
    for test, body in re.findall(r"fn (test_\w+)\(\) \{(.*?)\n        \}", source, re.S):
        for expression, expected in re.findall(
            r"assert_eq_const_safe!\(\$T: ([^,]+), ([^)]+)\);", body
        ):
            cases.append({"test": test, "expression": expression.strip(), "expected": expected.strip()})
    return cases
