# Purpose: run clippy, warnings as errors, on each crate a staged file is in.
# Never:   pass a Rust file that belongs to no crate; clippy cannot see it.
"""Usage: clippy_crates.py FILE... Exits 1 if clippy fails or a file has no crate."""

import subprocess
import sys
from pathlib import Path


def crate_of(path: Path) -> Path | None:
    """The nearest folder above `path`, inside the repo, holding a Cargo.toml."""
    for folder in path.resolve().parents:
        if not folder.is_relative_to(Path.cwd()):
            return None
        if (folder / "Cargo.toml").is_file():
            return folder
    return None


def clippy_passes(crate: Path) -> bool:
    """Runs clippy on every target of one crate."""
    command = [
        "cargo",
        "clippy",
        "--quiet",
        "--all-targets",
        "--manifest-path",
        str(crate / "Cargo.toml"),
        "--",
        "-D",
        "warnings",
    ]
    return subprocess.run(command, check=False).returncode == 0


def main(paths: list[str]) -> int:
    """Checks each crate once; 1 when any crate fails or a file has none."""
    crates = set()
    failed = False
    for name in paths:
        crate = crate_of(Path(name))
        if crate is None:
            print(f"{name}: belongs to no Cargo crate, clippy cannot check it")
            failed = True
            continue
        crates.add(crate)
    for crate in sorted(crates):
        failed = not clippy_passes(crate) or failed
    return int(failed)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
