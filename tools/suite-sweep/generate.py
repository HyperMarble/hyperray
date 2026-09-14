#!/usr/bin/env python3
# Write one program per extracted assertion. The assertion text belongs to the
# Rust suite, and black_box keeps the work in the machine code.
import json
import pathlib
import re
import sys

from extract import SUITE, extract
from identity import compare, identity, load


def substitute(text: str, constants: dict) -> str:
    """Longest name first, so BITS is never rewritten by a shorter match."""
    for name in sorted(constants, key=len, reverse=True):
        text = re.sub(rf"\b{name}\b", constants[name], text)
    return text.replace("$T", "i64")


def main() -> int:
    source = SUITE.read_text()
    cases = extract(source)
    directory = pathlib.Path(sys.argv[1])
    directory.mkdir(parents=True, exist_ok=True)
    previous = load(directory / "cases.json")
    written = []
    for case in cases:
        expression = substitute(case["expression"], case["constants"])
        expected = substitute(case["expected"], case["constants"])
        if "$" in expression or "$" in expected:
            continue
        name = identity(case["test"], expression, expected)
        (directory / f"{name}.rs").write_text(
            f"// {SUITE.name} {case['test']}: {case['expression']} == {case['expected']}\n"
            "// The assertion is the Rust suite's. black_box keeps the work in the\n"
            "// machine code so the proof examines instructions, not a constant.\n"
            "#![no_std]\n"
            "use core::hint::black_box;\n"
            '#[no_mangle]\n'
            'pub extern "C" fn _start(_input: u64) -> u64 {\n'
            f"    black_box(black_box({expression}) as u64)\n"
            "}\n"
        )
        written.append({"name": name, **case, "rust_expression": expression, "rust_expected": expected})
    (directory / "cases.json").write_text(json.dumps(written, indent=2))
    change = compare(previous, written)
    print(f"assertions extracted: {len(cases)}  programs written: {len(written)}")
    print(f"upstream change: {len(change['added'])} added, "
          f"{len(change['removed'])} removed, {len(change['unchanged'])} unchanged")
    return 0


if __name__ == "__main__":
    sys.exit(main())
