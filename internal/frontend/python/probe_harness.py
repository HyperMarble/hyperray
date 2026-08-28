import contextlib
import dis
import hashlib
import importlib
import io
import json
import os
from pathlib import Path
import sys


class AuditViolation(RuntimeError):
    pass


REQUEST = json.loads(__RAY_PAYLOAD__)


def snapshot(root):
    result = {}
    for directory, names, files in os.walk(root):
        names[:] = sorted(name for name in names if name != "__pycache__")
        for name in sorted(files):
            path = Path(directory) / name
            relative = str(path.relative_to(root))
            if relative == REQUEST["observation_path"]:
                continue
            result[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result


def import_symbol(path):
    module_name, separator, symbol_name = path.rpartition(".")
    if not separator:
        raise ValueError("constructor must be fully qualified")
    return getattr(importlib.import_module(module_name), symbol_name)


def decode_literal(literal, constructor=""):
    kind = literal["type"]
    if kind == "bool":
        return literal["bool"]
    if kind == "integer":
        return literal["integer"]
    if kind == "string":
        return literal["string"]
    if kind in ("unit", "optional") and (kind == "unit" or literal["null"]):
        return None
    if kind in ("sequence", "tuple"):
        child_constructor = constructor.split(":", 1)[1] if constructor.startswith("sequence:") else ""
        values = [decode_literal(item, child_constructor) for item in literal["elements"]["values"]]
        return tuple(values) if kind == "tuple" else values
    if kind == "record":
        values = {name: decode_literal(value) for name, value in literal["fields"]["values"].items()}
        return import_symbol(constructor)(**values) if constructor else values
    raise ValueError("unsupported probe literal " + kind)


def encode_literal(value):
    literal = {"type": "", "bool": False, "integer": 0, "string": "", "null": False}
    if value is None:
        literal.update({"type": "optional", "null": True})
    elif type(value) is bool:
        literal.update({"type": "bool", "bool": value})
    elif type(value) is int and -(2**63) <= value <= 2**63 - 1:
        literal.update({"type": "integer", "integer": value})
    elif type(value) is str:
        literal.update({"type": "string", "string": value})
    elif type(value) in (list, tuple):
        literal.update({
            "type": "tuple" if type(value) is tuple else "sequence",
            "elements": {"values": [encode_literal(item) for item in value]},
        })
    elif hasattr(value, "__dataclass_fields__"):
        literal.update({
            "type": "record",
            "fields": {"values": {name: encode_literal(getattr(value, name)) for name in value.__dataclass_fields__}},
        })
    else:
        raise ValueError("unsupported probe result type " + type(value).__name__)
    return literal


def main():
    root = Path(__file__).resolve().parents[2]
    package_root = (root / REQUEST["package_root"]).resolve()
    source_path = (root / REQUEST["source_path"]).resolve()
    if root != package_root and root not in package_root.parents:
        raise RuntimeError("package root escapes probe workspace")
    sys.path.insert(0, str(package_root))
    os.chdir(str(root))

    allowed_roots = (root, Path(sys.base_prefix).resolve())

    def inside_allowed(path):
        try:
            resolved = Path(path).resolve()
        except (TypeError, ValueError, OSError):
            return False
        return any(resolved == allowed or allowed in resolved.parents for allowed in allowed_roots)

    def audit(event, args):
        if event in {
            "ctypes.dlopen", "ctypes.dlsym", "ctypes.dlsym/handle",
            "os.posix_spawn", "os.posix_spawnp", "os.spawn", "os.system",
            "socket.__new__", "socket.bind", "socket.connect", "socket.getaddrinfo",
            "subprocess.Popen",
        }:
            raise AuditViolation("uncontrolled audit event: " + event)
        if event == "open" and args and not isinstance(args[0], int):
            mode = args[1] if len(args) > 1 else "r"
            observation = (root / REQUEST["observation_path"]).resolve()
            target = Path(args[0]).resolve()
            writing_observation = target == observation and any(flag in str(mode) for flag in ("w", "a", "+", "x"))
            if not writing_observation and (any(flag in str(mode) for flag in ("w", "a", "+", "x")) or not inside_allowed(target)):
                raise AuditViolation("uncontrolled file access: " + repr(args[0]))

    sys.addaudithook(audit)
    before = snapshot(root)
    import_out, import_err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(import_out), contextlib.redirect_stderr(import_err):
        module = importlib.import_module(REQUEST["module"])
    if import_out.getvalue() or import_err.getvalue():
        raise RuntimeError("probe import produced undeclared output")

    bytecode = {}
    for operation in sorted({case["operation"] for case in REQUEST["cases"]}):
        function = getattr(module, operation)
        bytecode[operation] = [
            (item.offset, item.opname, item.argrepr, item.starts_line, item.positions.lineno, item.positions.end_lineno)
            for item in dis.get_instructions(function, adaptive=False)
        ]
    emitted = "sha256:" + hashlib.sha256(json.dumps(bytecode, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    if emitted != REQUEST["bytecode_digest"]:
        raise RuntimeError("probe bytecode differs from translated compiler evidence")

    traces = []
    for case in REQUEST["cases"]:
        constructors = case["constructors"] or [""] * len(case["arguments"])
        arguments = [decode_literal(value, constructor) for value, constructor in zip(case["arguments"], constructors)]
        call_out, call_err = io.StringIO(), io.StringIO()
        try:
            with contextlib.redirect_stdout(call_out), contextlib.redirect_stderr(call_err):
                value = getattr(module, case["operation"])(*arguments)
        except AuditViolation:
            raise
        except Exception as exc:
            observed = {
                "kind": "raise",
                "exception_type": type(exc).__name__,
                "message": str(exc),
                "effects": None,
            }
        else:
            observed = {
                "kind": "return",
                "value": encode_literal(value),
                "exception_type": "",
                "message": "",
                "effects": None,
            }
        if call_out.getvalue() or call_err.getvalue():
            raise RuntimeError("probe case produced undeclared output")
        if observed != case["expected"]:
            raise RuntimeError("probe result differs for " + case["id"])
        traces.append(observed)

    if snapshot(root) != before:
        raise RuntimeError("probe mutated frozen workspace")
    observation = root / REQUEST["observation_path"]
    observation.parent.mkdir(parents=True, exist_ok=True)
    observation.write_text(json.dumps({"traces": traces}, sort_keys=True, separators=(",", ":")), encoding="utf-8")


if __name__ == "__main__":
    main()
