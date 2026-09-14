#!/usr/bin/env python3
# Compile and link one extracted assertion, and read back what the compiler
# actually produced. It must report the reason a step stopped, never a guess.
#
# 17 of 188 assertions call unstable APIs such as div_floor. The compiler names
# the exact feature each one needs, so the gate is read from its message and
# the build is retried. Copying the suite crate's whole feature list instead
# would break on any toolchain that has since renamed or removed one.
import os
import pathlib
import re
import subprocess


TOOLCHAIN = os.environ.get("SWEEP_TOOLCHAIN", "nightly")


def run(command: list) -> subprocess.CompletedProcess:
    return subprocess.run(command, capture_output=True, text=True)


def first_error(text: str) -> str:
    """The compiler's own first error line, so a block names its real cause."""
    for line in text.split("\n"):
        if line.startswith("error"):
            return line.strip()[:90]
    return "no error line reported"


def compile_object(directory: pathlib.Path, name: str) -> subprocess.CompletedProcess:
    return run([
        "rustup", "run", TOOLCHAIN,
        "rustc", "--edition=2021", "--target", "aarch64-apple-darwin",
        "--crate-type=lib", "--emit=obj", "-C", "panic=abort",
        "-C", "relocation-model=static", "-C", "codegen-units=1", "-O",
        str(directory / f"{name}.rs"), "-o", str(directory / f"{name}.o"),
    ])


def required_feature(stderr: str) -> str:
    """The feature the compiler says to add, taken from its own help line."""
    found = re.search(r"add `#!\[feature\((\w+)\)\]`", stderr)
    return found.group(1) if found else ""


def enable_feature(source: pathlib.Path, feature: str) -> None:
    text = source.read_text()
    source.write_text(text.replace("#![no_std]\n", f"#![no_std]\n#![feature({feature})]\n", 1))


def build(directory: pathlib.Path, name: str) -> str:
    """Returns an empty string on success, or the reason the build stopped."""
    sdk = run(["xcrun", "--sdk", "macosx", "--show-sdk-version"]).stdout.strip()
    compiled = compile_object(directory, name)
    feature = required_feature(compiled.stderr) if compiled.returncode else ""
    if feature:
        enable_feature(directory / f"{name}.rs", feature)
        compiled = compile_object(directory, name)
    if compiled.returncode != 0:
        return "rustc rejected the program: " + first_error(compiled.stderr)
    linked = run([
        "xcrun", "ld", "-arch", "arm64", "-static", "-no_pie",
        "-platform_version", "macos", sdk, sdk, "-e", "__start",
        "-undefined", "error", "-o", str(directory / f"{name}.bin"),
        str(directory / f"{name}.o"),
    ])
    if linked.returncode != 0:
        return "linker rejected the object"
    return ""
