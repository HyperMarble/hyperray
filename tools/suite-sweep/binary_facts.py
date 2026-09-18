#!/usr/bin/env python3
# Read the entry point, the extent, and the mapped segments from one binary.
# Every value comes from the linked artifact, never from a declaration.
import pathlib
import re

from build import run


def entry_and_end(binary: pathlib.Path, entry: str = "__start") -> tuple:
    """The named entry symbol and the address after its return instruction.

    The first text symbol is not the entry. A no_std program defines a panic
    handler that branches to itself, and it can sort first.
    """
    symbols = run(["xcrun", "nm", "-g", str(binary)]).stdout
    start = None
    for line in symbols.split("\n"):
        parts = line.split()
        if len(parts) == 3 and parts[1] == "T" and parts[2] == entry:
            start = int(parts[0], 16)
            break
    if start is None:
        return None, None
    listing = run(["xcrun", "objdump", "-d", str(binary)]).stdout
    rows = [
        (int(match.group(1), 16), match.group(2))
        for match in re.finditer(r"^\s*([0-9a-f]{6,}):\s+[0-9a-f]{8}\s+(\S+)", listing, re.M)
    ]
    for address, operation in [row for row in rows if row[0] >= start]:
        if operation == "ret":
            return start, address + 4
    return start, None

def segments(listing: str) -> list:
    """Every segment otool reports, with the protection it declares."""
    found, name, address, size = [], "", None, None
    for line in listing.split("\n"):
        text = line.strip()
        if text.startswith("segname"):
            name, address, size = text.split()[1], None, None
        if text.startswith("vmaddr"):
            address = int(text.split()[1], 16)
        if text.startswith("vmsize"):
            size = int(text.split()[1], 16)
        if text.startswith("initprot") and address is not None and size is not None:
            found.append({"name": name, "address": address, "size": size,
                          "initprot": int(text.split()[1], 16)})
            address, size = None, None
    return found


def loadable(segment: dict) -> bool:
    """A segment the proof must map: it holds bytes at a real address."""
    return segment["name"] != "__PAGEZERO" and segment["size"] > 0


def region(segment: dict) -> dict:
    """One mapping, with the protection the segment states.

    The name does not decide this: __DATA_CONST is read and write, and
    naming it read-only leaves its bytes uncovered.
    """
    protection = segment["initprot"]
    if protection & 0x4:
        permission = "RX"
    elif protection & 0x2:
        permission = "RW"
    else:
        permission = "R"
    return {"va": segment["address"], "pa": segment["address"],
            "length": segment["size"], "permission": permission}


def mappings(binary: pathlib.Path) -> list:
    """The mapped regions of the binary, plus the scratch page the proof uses."""
    listing = run(["xcrun", "otool", "-l", str(binary)]).stdout
    seen, unique = set(), []
    for segment in segments(listing):
        if not loadable(segment) or segment["address"] in seen:
            continue
        seen.add(segment["address"])
        unique.append(region(segment))
    unique.append({"va": 0x3000, "pa": 0x3000, "length": 0x1000, "permission": "RW"})
    return unique
