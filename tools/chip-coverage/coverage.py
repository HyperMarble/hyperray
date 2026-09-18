#!/usr/bin/env python3
# Compare the chip's features against what the proof model covers.
#
#   python3 coverage.py <sail.ir> <isla-config.toml>
#
# Three buckets, each a measured fact: covered, present in the model but
# switched off, and absent from the model. A feature with no known ARM
# version is listed as unknown rather than placed in a bucket.
import sys

import chip
from feature_version import introduced_in
from model import enabled_version, ir_version


def bucket(feature: str, enabled: str, in_ir: str) -> str:
    version = introduced_in(feature)
    if not version:
        return "unknown version"
    if version <= enabled:
        return "covered"
    if version <= in_ir:
        return "in model, switched off"
    return "not in model"


def main() -> int:
    enabled = enabled_version(sys.argv[2])
    in_ir = ir_version(sys.argv[1])
    groups = {}
    for feature in chip.features():
        groups.setdefault(bucket(feature, enabled, in_ir), []).append(feature)
    print(f"chip: {chip.name()}")
    print(f"model IR: ARMv{in_ir}   config enables up to: ARMv{enabled}\n")
    for key in ("covered", "in model, switched off", "not in model", "unknown version"):
        names = groups.get(key, [])
        print(f"{key}: {len(names)}")
        if names:
            print("  " + " ".join(names))
    return 0


if __name__ == "__main__":
    sys.exit(main())
