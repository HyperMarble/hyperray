#!/usr/bin/env python3
# Check the extraction against the compiler, before any proof runs.
#
# Every extracted assertion must hold in real Rust. A substitution mistake
# makes the assertion false, and rustc says so: the flat-constant defect
# built i64::MIN.wrapping_pow(1) where the suite wrote 0, and this check is
# what caught it. The compiler decides, not a rule restated here.
import json
import pathlib
import sys

from build import TOOLCHAIN, required_feature, run


def program(cases: list) -> str:
    """One Rust program that evaluates every assertion and counts failures."""
    lines = [
        f'    if ({case["rust_expression"]}) as i64 != ({case["rust_expected"]}) as i64 '
        f'{{ println!("FAIL {case["test"]} line {case["line"]}: {case["rust_expression"]}"); '
        "bad += 1; }"
        for case in cases
    ]
    return ("fn main() {\n    let mut bad = 0;\n" + "\n".join(lines) +
            f'\n    println!("checked {len(cases)} assertions, failures: {{}}", bad);\n}}\n')


def compile_with_gates(source: pathlib.Path, output: pathlib.Path):
    """Retry with the feature the compiler names, up to once per feature."""
    result = run(["rustup", "run", TOOLCHAIN, "rustc", "--edition=2021", str(source), "-o", str(output)])
    seen = set()
    feature = required_feature(result.stderr)
    while feature and feature not in seen:
        seen.add(feature)
        source.write_text(f"#![feature({feature})]\n" + source.read_text())
        result = run(["rustup", "run", TOOLCHAIN, "rustc", "--edition=2021", str(source), "-o", str(output)])
        feature = required_feature(result.stderr)
    return result


def main() -> int:
    directory = pathlib.Path(sys.argv[1])
    cases = json.loads((directory / "cases.json").read_text())
    source = directory / "checkall.rs"
    source.write_text(program(cases))
    compiled = compile_with_gates(source, directory / "checkall")
    if compiled.returncode != 0:
        print("the extraction does not compile:")
        print("\n".join(compiled.stderr.split("\n")[:12]))
        return 1
    outcome = run([str(directory / "checkall")])
    print(outcome.stdout.strip())
    return 0 if "failures: 0" in outcome.stdout else 1


if __name__ == "__main__":
    sys.exit(main())
