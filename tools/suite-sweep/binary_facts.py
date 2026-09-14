#!/usr/bin/env python3
# Read the entry point, the extent, and the mapped segments from one binary.
# Every value comes from the linked artifact, never from a declaration.
import pathlib
import re

from build import run


def entry_and_end(binary: pathlib.Path) -> tuple:
    """The entry symbol and the address after its return instruction."""
    symbols = run(["xcrun", "nm", "-g", str(binary)]).stdout
    start = None
    for line in symbols.split("\n"):
        parts = line.split()
        if len(parts) == 3 and parts[1] == "T":
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
    """Every segment otool reports, as a name with its address and size."""
    found, name, address = [], "", None
    for line in listing.split("\n"):
        text = line.strip()
        if text.startswith("segname"):
            name, address = text.split()[1], None
        if text.startswith("vmaddr"):
            address = int(text.split()[1], 16)
        if text.startswith("vmsize") and address is not None:
            found.append({"name": name, "address": address, "size": int(text.split()[1], 16)})
            address = None
    return found


def loadable(segment: dict) -> bool:
    """A segment the proof must map: it holds bytes at a real address."""
    return segment["name"] != "__PAGEZERO" and segment["size"] > 0


def region(segment: dict) -> dict:
    permission = {"__TEXT": "RX", "__DATA": "RW"}.get(segment["name"], "R")
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
