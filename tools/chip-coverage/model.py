#!/usr/bin/env python3
# What the Sail model we prove against covers. Two facts, both read from
# files: the IR's architecture version, and which versions the config
# switches on. The IR may hold rules the config refuses to use.
import pathlib
import re


def ir_version(ir_path: str) -> str:
    """The ARM version the IR was built for, taken from its file name."""
    found = re.search(r"armv(\d)p(\d)", pathlib.Path(ir_path).name)
    return f"{found.group(1)}.{found.group(2)}" if found else ""


def enabled_version(config_path: str) -> str:
    """The highest ARM version the config switches on."""
    text = pathlib.Path(config_path).read_text()
    highest = "8.0"
    for minor, state in re.findall(r'"__v8(\d)_implemented"\s*=\s*(true|false)', text):
        if state == "true":
            highest = max(highest, f"8.{minor}")
    return highest
