#!/usr/bin/env python3
# Build one proof request from measured binary facts. The claim is given by
# the caller; this file never decides what is asserted.
import os
import pathlib

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
RETURN_ADDRESS = 0x100010000


def register(name: str, value: int) -> str:
    """A claim that one register holds one value."""
    return f"0:{name} = 0x{value & 0xFFFFFFFFFFFFFFFF:016x}"


def memory(name: str, value: int) -> str:
    """A claim that one named memory observation holds one value."""
    return f"*{name} = 0x{value & 0xFFFFFFFFFFFFFFFF:016x}"


def returned(value: int) -> str:
    """A claim that the function returned `value` and reached its caller."""
    return f"({register('R0', value)} & {register('_PC', RETURN_ADDRESS)})"


def request(binary: pathlib.Path, name: str, start: int, end: int,
            claim: str, observations: list = None) -> dict:
    """One request. `claim` is asserted as given, never rewritten here."""
    tools = {key: os.environ[variable] for key, variable in TOOLS.items()}
    tools["architecture_sha256"] = ARCHITECTURE_DIGEST
    tools["manifest_sha256"] = os.environ["HYPERRAY_ARM64_NORMAL_EXECUTION_MANIFEST_SHA256"]
    return {
        "binary": str(binary.resolve()),
        "tools": tools,
        "boundary": {
            "name": f"suite-{name}", "function_start": start, "function_end": end,
            "return_address": RETURN_ADDRESS,
            "post_reset_registers": [
                {"name": "R0", "value": "0x0000000000000000"},
                {"name": "R30", "value": f"0x{RETURN_ADDRESS:016x}"},
                {"name": "SP_EL0", "value": "0x0000000000003c40"},
            ],
            "memory_observations": observations or [],
        },
        "memory": {
            "table_base": 0x5000, "table_capacity_pages": 256,
            "mappings": mappings(binary),
            "backing": [{"address": 0x3B00, "permission": "RW", "bytes": "00" * 320}],
        },
        "negated_assertion": f"~({claim})",
        "limits": {
            "maximum_loaded_bytes": 65536, "maximum_output_bytes": 134217728,
            "time_limit_seconds": 120, "pc_visit_limit": 512,
        },
    }
