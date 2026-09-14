#!/usr/bin/env python3
# Write one program per extracted assertion. The assertion text belongs to the
# Rust suite, and black_box keeps the work in the machine code.
import json
import pathlib
import re
import sys

from extract import CONSTANTS, SUITE, extract, read_constants


def substitute(text: str) -> str:
    for name, value in CONSTANTS.items():
        text = re.sub(rf"\b{name}\b", value, text)
    return text.replace("$T", "i64")


def main() -> int:
    source = SUITE.read_text()
    CONSTANTS.update(read_constants(source))
    cases = extract(source)
    directory = pathlib.Path(sys.argv[1])
    directory.mkdir(parents=True, exist_ok=True)
    written = []
    for index, case in enumerate(cases):
        expression = substitute(case["expression"])
        expected = substitute(case["expected"])
        if "$" in expression or "$" in expected:
            continue
        name = f"s{index:03d}"
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
    print(f"constants: {len(CONSTANTS)}  assertions extracted: {len(cases)}  programs written: {len(written)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
