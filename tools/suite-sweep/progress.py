#!/usr/bin/env python3
# Progress reporting names the stage each assertion is in, and the elapsed
# time. It must report the stage that is running, never one already finished.
#
# A terminal redraws one line in place. A log file cannot, so it gets one line
# per stage instead. A sweep runs for hours in the background, and a log that
# collapses to a single unreadable line loses the record of where time went.
import sys
import time


class Progress:
    """Reports one line per assertion, rewritten as its stage advances."""

    def __init__(self, total: int, done: int):
        self.total = total
        self.finished = done
        self.started = 0.0
        self.label = ""
        self.redraws = sys.stderr.isatty()

    def begin(self, position: int, name: str, test: str) -> None:
        self.started = time.monotonic()
        self.label = f"[{position}/{self.total}] {name} {test}"
        self.stage("starting")

    def stage(self, stage: str) -> None:
        elapsed = time.monotonic() - self.started
        line = f"{self.label:52s} {stage:22s} {elapsed:5.1f}s"
        prefix, suffix = ("\r\033[K", "") if self.redraws else ("", "\n")
        sys.stderr.write(prefix + line + suffix)
        sys.stderr.flush()

    def finish(self, result: str) -> None:
        elapsed = time.monotonic() - self.started
        self.finished += 1
        if self.redraws:
            sys.stderr.write("\r\033[K")
            sys.stderr.flush()
        print(f"{self.label:52s} {result:34s} {elapsed:5.1f}s", flush=True)

    def summary(self, results: list) -> None:
        counts = {}
        for entry in results:
            key = entry["result"].split(":")[0]
            counts[key] = counts.get(key, 0) + 1
        parts = [f"{key} {value}" for key, value in sorted(counts.items())]
        print(f"\n{len(results)} of {self.total}: " + ", ".join(parts), flush=True)
