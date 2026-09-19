#!/usr/bin/env python3
# Build one proof request from measured binary facts. The claim is given by
# the caller; this file never decides what is asserted.
import hashlib
import os
import pathlib

from binary_facts import mappings

HYPERRAY = "/Volumes/Hak_SSD/hyperray-build/hyperray"
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


def loaded_byte_budget(placed: list) -> int:
    """Room for every byte these mappings cover.

    A no_std test maps two pages; a std binary maps hundreds. A fixed budget
    refuses the larger one before any proof starts.
    """
    return sum(entry["length"] for entry in placed)


def arena_pages(placed: list) -> int:
    """The page-table arena the engine requires for these mappings.

    The engine checks four table pages for every mapped page, plus a root.
    A fixed number is wrong for any binary that maps a different amount.
    """
    mapped = sum(entry["length"] for entry in placed) // 0x1000
    return 1 + 4 * mapped


def measured(path: str) -> str:
    """The digest of the file as it is now, so a request pins what it reads."""
    value = hashlib.sha256()
    with open(path, "rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


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
    placed = mappings(binary)
    tools = {key: os.environ[variable] for key, variable in TOOLS.items()}
    tools["architecture_sha256"] = measured(tools["architecture"])
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
            "table_base": 0x5000, "table_capacity_pages": arena_pages(placed),
            "mappings": placed,
            "backing": [{"address": 0x3B00, "permission": "RW", "bytes": "00" * 320}],
        },
        "negated_assertion": f"~({claim})",
        "limits": {
            "maximum_loaded_bytes": loaded_byte_budget(placed), "maximum_output_bytes": 134217728,
            "time_limit_seconds": 120, "pc_visit_limit": 512,
        },
    }
