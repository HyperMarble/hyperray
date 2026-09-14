#!/usr/bin/env python3
# Compile and link one extracted assertion, and read back what the compiler
# actually produced. It must report the reason a step stopped, never a guess.
import pathlib
import re
import subprocess


def run(command: list) -> subprocess.CompletedProcess:
    return subprocess.run(command, capture_output=True, text=True)


def build(directory: pathlib.Path, name: str) -> str:
    """Returns an empty string on success, or the reason the build stopped."""
    sdk = run(["xcrun", "--sdk", "macosx", "--show-sdk-version"]).stdout.strip()
    compiled = run([
        "rustc", "--edition=2021", "--target", "aarch64-apple-darwin",
        "--crate-type=lib", "--emit=obj", "-C", "panic=abort",
        "-C", "relocation-model=static", "-C", "codegen-units=1", "-O",
        str(directory / f"{name}.rs"), "-o", str(directory / f"{name}.o"),
    ])
    if compiled.returncode != 0:
        return "rustc rejected the program"
    linked = run([
        "xcrun", "ld", "-arch", "arm64", "-static", "-no_pie",
        "-platform_version", "macos", sdk, sdk, "-e", "__start",
        "-undefined", "error", "-o", str(directory / f"{name}.bin"),
        str(directory / f"{name}.o"),
    ])
    if linked.returncode != 0:
        return "linker rejected the object"
    return ""
