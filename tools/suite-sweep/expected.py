#!/usr/bin/env python3
# Evaluate the suite's expected value by running it, never by computing it
# here. The value is whatever real Rust prints on the same toolchain that
# built the machine code being proved.
import pathlib

from build import TOOLCHAIN, required_feature, run


def compile_probe(probe: pathlib.Path, output: pathlib.Path):
    return run(["rustup", "run", TOOLCHAIN, "rustc", "--edition=2021", "-O",
                str(probe), "-o", str(output)])


def expected_value(directory: pathlib.Path, name: str, expression: str) -> tuple:
    """Evaluates the suite's expected value with rustc, not with our own rules.

    An expected value can itself use an unstable API, so the probe takes the
    same feature gate the compiler asks for, on the same toolchain.
    """
    probe = directory / f"{name}_expected.rs"
    output = directory / f"{name}_expected"
    probe.write_text(
        "#![allow(unused)]\n"
        "fn main() {\n"
        f"    println!(\"{{}}\", ({expression}) as i64);\n"
        "}\n"
    )
    compiled = compile_probe(probe, output)
    feature = required_feature(compiled.stderr) if compiled.returncode else ""
    if feature:
        probe.write_text(f"#![feature({feature})]\n" + probe.read_text())
        compiled = compile_probe(probe, output)
    if compiled.returncode != 0:
        return None, "rustc could not evaluate the expected value: " + compiled.stderr.strip().split("\n")[0][:70]
    output = run([str(directory / f"{name}_expected")])
    if output.returncode != 0:
        return None, "the expected value panicked"
    return int(output.stdout.strip()), ""
