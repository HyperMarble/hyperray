#!/usr/bin/env python3
# Extract assertions from the Rust integer test suite and prove each one
# against real machine code. The assertion text is Rust's, never ours.
import pathlib
import re

from scope import scoped_constants

LIBRARY = pathlib.Path(
    "/Users/hak/.rustup/toolchains/stable-aarch64-apple-darwin/lib/rustlib/src/rust/library"
)
SUITE = LIBRARY / "coretests/tests/num/int_macros.rs"
ASSERTION = re.compile(r"assert_eq_const_safe!\(\$T: ([^,]+), ([^)]+)\);")


def extract(source: str) -> list:
    """One entry per assertion, carrying the constants visible at its line."""
    lines = source.split("\n")
    visible = scoped_constants(source)
    cases = []
    test = ""
    for number, line in enumerate(lines):
        named = re.search(r"fn (test_\w+)\(\)", line)
        if named:
            test = named.group(1)
        found = ASSERTION.search(line)
        if found and test:
            cases.append({"test": test, "expression": found.group(1).strip(),
                          "expected": found.group(2).strip(),
                          "constants": visible[number], "line": number + 1})
    return cases
