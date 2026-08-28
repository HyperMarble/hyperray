"""Run every probe script in one interpreter, paying the import cost once.

Each script still runs completely and independently -- same code, same
assertions -- only the process boundary is removed. Output sections are
delimited so the engine can split them back into per-probe observations.
"""
import contextlib
import io
import runpy
import sys

for path in sys.argv[1:]:
    buffer = io.StringIO()
    try:
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(buffer):
            runpy.run_path(path, run_name="__main__")
    except SystemExit:
        pass
    except Exception as error:  # noqa: BLE001 - a probe error is an observation
        buffer.write(f"{type(error).__name__}: {error}")
    name = path.rsplit("/", 1)[-1]
    sys.stdout.write(f"===RAY_PROBE {name}===\n{buffer.getvalue()}\n")
