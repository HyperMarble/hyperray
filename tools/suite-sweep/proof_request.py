#!/usr/bin/env python3
# Build one proof request from measured binary facts and the suite's own
# expected value. It must not invent a boundary the binary does not show.
import os
import pathlib
import subprocess

from build import run
from binary_facts import mappings

HYPERRAY = "/Volumes/Hak_SSD/hyperray-build/hyperray"
ARCHITECTURE_DIGEST = "4ece4410a43c32b58737957d55920b485ade5171bb53deddebb9c7d61df77075"
TOOLS = {
    "solver": "HYPERRAY_ARM64_ISLA",
    "semantics": "HYPERRAY_ARM64_ISLA_DUMP",
    "footprints": "HYPERRAY_ARM64_ISLA_FOOTPRINT",
    "architecture": "HYPERRAY_ARM64_SAIL_IR",
    "configuration": "HYPERRAY_ARM64_ISLA_CONFIG",
    "memory_model": "HYPERRAY_ARM64_MEMORY_MODEL",
    "manifest": "HYPERRAY_ARM64_NORMAL_EXECUTION_MANIFEST",
}


def request(binary: pathlib.Path, name: str, start: int, end: int, expected: int) -> dict:
    tools = {key: os.environ[variable] for key, variable in TOOLS.items()}
    tools["architecture_sha256"] = ARCHITECTURE_DIGEST
    tools["manifest_sha256"] = os.environ["HYPERRAY_ARM64_NORMAL_EXECUTION_MANIFEST_SHA256"]
    return {
        "binary": str(binary.resolve()),
        "tools": tools,
        "boundary": {
            "name": f"suite-{name}", "function_start": start, "function_end": end,
            "return_address": 0x100010000,
            "post_reset_registers": [
                {"name": "R0", "value": "0x0000000000000000"},
                {"name": "R30", "value": "0x0000000100010000"},
                {"name": "SP_EL0", "value": "0x0000000000003c40"},
            ],
            "memory_observations": [],
        },
        "memory": {
            "table_base": 0x5000, "table_capacity_pages": 256,
            "mappings": mappings(binary),
            "backing": [{"address": 0x3B00, "permission": "RW", "bytes": "00" * 320}],
        },
        "negated_assertion":
            f"~((0:R0 = 0x{expected & 0xFFFFFFFFFFFFFFFF:016x} & 0:_PC = 0x0000000100010000))",
        "limits": {
            "maximum_loaded_bytes": 65536, "maximum_output_bytes": 134217728,
            "time_limit_seconds": 120, "pc_visit_limit": 512,
        },
    }

def expected_value(directory: pathlib.Path, name: str, expression: str) -> tuple:
    """Evaluates the suite's expected value with rustc, not with our own rules."""
    probe = directory / f"{name}_expected.rs"
    probe.write_text(
        "fn main() {\n"
        f"    println!(\"{{}}\", ({expression}) as i64);\n"
        "}\n"
    )
    compiled = run(["rustc", "--edition=2021", "-O", str(probe), "-o", str(directory / f"{name}_expected")])
    if compiled.returncode != 0:
        return None, "rustc could not evaluate the expected value"
    output = run([str(directory / f"{name}_expected")])
    if output.returncode != 0:
        return None, "the expected value panicked"
    return int(output.stdout.strip()), ""
