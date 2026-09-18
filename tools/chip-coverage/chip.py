#!/usr/bin/env python3
# Ask the running chip which ARM features it has. The list comes from the
# kernel, never from a table, so a new chip reports itself.
import re
import subprocess


def features() -> list:
    """Every ARM feature the local chip reports as present."""
    output = subprocess.run(["sysctl", "-a"], capture_output=True, text=True).stdout
    found = re.findall(r"hw\.optional\.arm\.(\S+): 1", output)
    return sorted(name for name in found if not name.startswith("caps"))


def name() -> str:
    return subprocess.run(["sysctl", "-n", "machdep.cpu.brand_string"],
                          capture_output=True, text=True).stdout.strip()
