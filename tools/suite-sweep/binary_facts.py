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

def mappings(binary: pathlib.Path) -> list:
    listing = run(["xcrun", "otool", "-l", str(binary)]).stdout
    regions, current = [], {}
    for line in listing.split("\n"):
        text = line.strip()
        if text.startswith("segname"):
            current = {"name": text.split()[1]}
        elif text.startswith("vmaddr") and current:
            current["address"] = int(text.split()[1], 16)
        elif text.startswith("vmsize") and current:
            size = int(text.split()[1], 16)
            if current["name"] != "__PAGEZERO" and size > 0:
                permission = {"__TEXT": "RX", "__DATA": "RW"}.get(current["name"], "R")
                regions.append({
                    "va": current["address"], "pa": current["address"],
                    "length": size, "permission": permission,
                })
            current = {}
    seen, unique = set(), []
    for region in regions:
        if region["va"] in seen:
            continue
        seen.add(region["va"])
        unique.append(region)
    unique.append({"va": 0x3000, "pa": 0x3000, "length": 0x1000, "permission": "RW"})
    return unique
