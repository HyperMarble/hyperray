"""Exhaustively execute declared finite Python assignments.

Input is produced only by the Go frontend from typed Semantic IR domains.  The
runner starts in a fresh process, imports the frozen workspace module, invokes
every supplied case, records a typed result and target-source terminal line,
and rejects stdout/stderr or filesystem mutation.  The Go caller repeats the
entire run in reverse case order and requires identical per-case evidence.
"""

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


def snapshot(root):
    result = {}
    for directory, names, files in os.walk(root):
        names[:] = sorted(name for name in names if name != "__pycache__")
        for name in sorted(files):
            path = Path(directory) / name
            relative = str(path.relative_to(root))
            try:
                content = path.read_bytes()
            except OSError as exc:
                raise RuntimeError("cannot snapshot " + relative + ": " + str(exc))
            result[relative] = hashlib.sha256(content).hexdigest()
    return result


def import_symbol(path):
    module_name, separator, symbol_name = path.rpartition(".")
    if not separator:
        raise ValueError("constructor must be a fully qualified symbol")
    return getattr(importlib.import_module(module_name), symbol_name)


def decode_literal(literal, constructor=""):
    kind = literal.get("type", "")
    if kind == "bool":
        return bool(literal.get("bool", False))
    if kind == "integer":
        return int(literal.get("integer", 0))
    if kind == "string":
        return literal.get("string", "")
    if kind in ("unit", "optional"):
        if kind == "optional" and not literal.get("null", False):
            raise ValueError("non-null optional values require a typed inner representation")
        return None
    if kind in ("sequence", "tuple"):
        elements = (literal.get("elements") or {}).get("values", [])
        child_constructor = ""
        if constructor.startswith("sequence:"):
            child_constructor = constructor.split(":", 1)[1]
        values = [decode_literal(item, child_constructor) for item in elements]
        return tuple(values) if kind == "tuple" else values
    if kind == "record":
        fields = (literal.get("fields") or {}).get("values", {})
        values = {name: decode_literal(value) for name, value in fields.items()}
        if constructor:
            return import_symbol(constructor)(**values)
        return values
    raise ValueError("unsupported typed literal " + repr(kind))


def encode_literal(value):
    if value is None:
        return {"type": "optional", "null": True}
    if type(value) is bool:
        return {"type": "bool", "bool": value}
    if type(value) is int:
        if value < -(2**63) or value > 2**63 - 1:
            raise ValueError("integer outcome exceeds Semantic IR int64")
        return {"type": "integer", "integer": value}
    if type(value) is str:
        return {"type": "string", "string": value}
    if type(value) in (list, tuple):
        return {
            "type": "tuple" if type(value) is tuple else "sequence",
            "elements": {"values": [encode_literal(item) for item in value]},
        }
    if hasattr(value, "__dataclass_fields__"):
        fields = {name: encode_literal(getattr(value, name)) for name in value.__dataclass_fields__}
        return {"type": "record", "fields": {"values": fields}}
    raise ValueError("outcome has unsupported runtime type " + type(value).__name__)


def full_literal(literal):
    kind = literal.get("type", "")
    result = {
        "type": kind,
        "bool": bool(literal.get("bool", False)),
        "integer": int(literal.get("integer", 0)),
        "string": literal.get("string", ""),
        "null": bool(literal.get("null", False)),
    }
    if kind in ("sequence", "tuple"):
        values = (literal.get("elements") or {}).get("values", [])
        result["elements"] = {"values": [full_literal(item) for item in values]}
    if kind == "record":
        values = (literal.get("fields") or {}).get("values", {})
        result["fields"] = {"values": {name: full_literal(values[name]) for name in sorted(values)}}
    return result


def canonical_json(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


def raw_outcome(result):
    value = full_literal(result["value"]) if result["kind"] == "return" else None
    return {
        "kind": result["kind"],
        **({"value": value} if value is not None else {}),
        "exception_type": result.get("exception_type", ""),
        "message": result.get("message", ""),
        "effects": None,
    }


def main():
    request = json.load(sys.stdin)
    root = Path(request["root"]).resolve()
    package_root = (root / request.get("package_root", ".")).resolve()
    if root != package_root and root not in package_root.parents:
        raise RuntimeError("package root escapes frozen workspace")
    before = snapshot(root)
    signal_path = (root / request.get("signal_path", "")).resolve()
    if not request.get("signal_path") or (signal_path != root and root not in signal_path.parents):
        raise RuntimeError("typed signal path escapes frozen workspace")
    if signal_path.exists():
        raise RuntimeError("typed signal path already exists")
    sys.path.insert(0, str(package_root))
    os.chdir(str(package_root))

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
            raise AuditViolation("uncontrolled audit event is forbidden: " + event)
        if event == "open" and args:
            target = args[0]
            mode = args[1] if len(args) > 1 else "r"
            if not isinstance(target, int):
                resolved_target = Path(target).resolve()
                protocol_write = resolved_target == signal_path and any(flag in str(mode) for flag in ("w", "x"))
                if (any(flag in str(mode) for flag in ("w", "a", "+", "x")) and not protocol_write) or not inside_allowed(target):
                    raise AuditViolation("uncontrolled file access is forbidden: " + repr(target))

    sys.addaudithook(audit)

    import_out, import_err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(import_out), contextlib.redirect_stderr(import_err):
        module = importlib.import_module(request["module"])
    if import_out.getvalue() or import_err.getvalue():
        raise RuntimeError("module import produced undeclared output")

    cases = list(request["cases"])
    if request.get("reverse"):
        cases.reverse()
    source_path = str((root / request["source_path"]).resolve())
    results = []
    raw_signals = []
    bytecode = {}
    declaration_nodes = {}
    declaration_opcodes = {}
    compiler_ir = {}
    declarations = request.get("declarations") or request.get("operations", [])
    for operation in declarations:
        function = getattr(module, operation)
        instructions = list(dis.get_instructions(function, adaptive=False))
        bytecode[operation] = [
            (item.offset, item.opname, item.argrepr, item.starts_line, item.positions.lineno, item.positions.end_lineno)
            for item in instructions
        ]
        declaration_nodes[operation] = [operation + ":" + str(item.offset) for item in instructions]
        declaration_opcodes[operation] = [item.opname for item in instructions]
        compiler_ir[operation] = [
            {
                "id": operation + ":" + str(item.offset),
                "offset": item.offset,
                "opcode": item.opname,
                "argument": item.argrepr,
                "line": int(item.positions.lineno or item.starts_line or function.__code__.co_firstlineno),
                "end_line": int(item.positions.end_lineno or item.positions.lineno or function.__code__.co_firstlineno),
            }
            for item in instructions
        ]
    for case in cases:
        function = getattr(module, case["operation"])
        if case["operation"] not in bytecode:
            instruction_items = list(dis.get_instructions(function, adaptive=False))
            instructions = [
                (item.offset, item.opname, item.argrepr, item.starts_line, item.positions.lineno, item.positions.end_lineno)
                for item in instruction_items
            ]
            bytecode[case["operation"]] = instructions
            declaration_nodes[case["operation"]] = [
                case["operation"] + ":" + str(item.offset) for item in instruction_items
            ]
            declaration_opcodes[case["operation"]] = [item.opname for item in instruction_items]
        constructors = list(case.get("constructors") or [])
        if not constructors:
            constructors = [""] * len(case["arguments"])
        if len(constructors) != len(case["arguments"]):
            raise RuntimeError("case " + case["id"] + " constructor arity mismatch")
        arguments = [
            decode_literal(literal, constructor)
            for literal, constructor in zip(case["arguments"], constructors)
        ]
        terminal_line = 0
        compiler_nodes = []

        declaration_set = set(declarations)

        def trace(frame, event, arg):
            nonlocal terminal_line
            if frame.f_code.co_name in declaration_set and str(Path(frame.f_code.co_filename).resolve()) == source_path:
                frame.f_trace_opcodes = True
                if frame.f_code.co_name == case["operation"] and event in ("line", "return", "exception"):
                    terminal_line = frame.f_lineno
                if event == "opcode":
                    compiler_nodes.append(frame.f_code.co_name + ":" + str(frame.f_lasti))
            return trace

        call_out, call_err = io.StringIO(), io.StringIO()
        sys.settrace(trace)
        try:
            with contextlib.redirect_stdout(call_out), contextlib.redirect_stderr(call_err):
                value = function(*arguments)
        except AuditViolation:
            raise
        except Exception as exc:
            result = {"kind": "raise", "exception_type": type(exc).__name__, "message": str(exc)}
        else:
            # Encoding is evidence validation, not part of the target call.
            # An unsupported result must block instead of being relabeled as
            # a target-thrown ValueError.
            result = {"kind": "return", "value": encode_literal(value)}
        finally:
            sys.settrace(None)
        if call_out.getvalue() or call_err.getvalue():
            raise RuntimeError("case " + case["id"] + " produced undeclared output")
        result.update({"id": case["id"], "line": terminal_line, "compiler_node_ids": sorted(set(compiler_nodes))})
        results.append(result)
        raw_signals.append(raw_outcome(result))

    after = snapshot(root)
    if before != after:
        raise RuntimeError("finite execution mutated the frozen workspace")
    if len(raw_signals) != 1:
        raise RuntimeError("typed signal protocol requires exactly one concrete case")
    signal_path.write_bytes(canonical_json(raw_signals[0]))
    bytecode_digest = hashlib.sha256(json.dumps(bytecode, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    # Preserve the actual execution order.  The Go frontend compares a
    # separately normalized semantic map across the forward and reverse fresh
    # processes, while retaining this ordered transcript as evidence that the
    # repeat really used an independent order.
    json.dump({
        "results": results,
        "bytecode_digest": "sha256:" + bytecode_digest,
        "compiler_node_ids": declaration_nodes,
        "compiler_opcodes": declaration_opcodes,
        "compiler_ir": compiler_ir,
    }, sys.stdout, separators=(",", ":"))


if __name__ == "__main__":
    main()
