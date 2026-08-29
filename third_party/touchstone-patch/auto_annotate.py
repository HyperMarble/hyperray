"""Auto-annotates unmodeled calls with their real declared return type,
resolved by mypy -- instead of requiring whoever writes the model file to
remember to annotate `x: bool = f(...)` by hand and trust it's correct.

Earlier version of this file hand-walked PEP 561 stub ASTs itself
(typeshed_client) to resolve a call's return type, including following
re-exports and class inheritance. That was the same brute-forcing this
session kept getting called out for, just moved into stub resolution: a
real, complete type checker already solves inheritance, generics,
overloads, and re-exports correctly, so this asks mypy directly (via
`reveal_type`) instead of reimplementing a fraction of one.

Deliberately narrow, same posture as hyperray-typed.patch itself: only
auto-annotates when mypy's revealed type is EXACTLY a bare `bool`/`int`/
`float` (accepting mypy's fully-qualified `builtins.X` spelling too).
Anything else -- `Any`, a union, an unresolvable attribute mypy itself
errors on -- is left unannotated. The existing patch (visit_AnnAssign +
the __ray_typed__ marker in ev()) still requires an annotation to act;
this module only tries to source one automatically before falling back
to whatever a human already wrote.
"""
import ast
import re
import subprocess
import sys
import tempfile
import os

_SCALARS = {"bool", "int", "float", "builtins.bool", "builtins.int", "builtins.float"}
_REVEAL_RE = re.compile(r'note: Revealed type is "([^"]+)"')


def _candidate_assigns(mod: ast.Module):
    """Every `x = f(...)` at any nesting depth, in source order."""
    for node in ast.walk(mod):
        if (isinstance(node, ast.Assign) and len(node.targets) == 1
                and isinstance(node.targets[0], ast.Name) and isinstance(node.value, ast.Call)):
            yield node


def auto_annotate(src: str, python_executable: str | None = None) -> str:
    """Rewrites `x = f(...)` to `x: T = f(...)` wherever mypy resolves f's
    return type to exactly bool/int/float. One mypy invocation covers every
    candidate in the file. Returns src unchanged if nothing resolves, mypy
    isn't available, or the source doesn't parse -- always safe to call."""
    try:
        mod = ast.parse(src)
    except SyntaxError:
        return src

    candidates = list(_candidate_assigns(mod))
    if not candidates:
        return src

    lines = src.splitlines(keepends=True)
    # Insert `reveal_type(x)` right after each candidate's own line, tagged
    # with its list index in a marker comment so output lines are matched
    # back to a specific assignment even if several reveal on the same line.
    inserted = 0
    annotated_lines = list(lines)
    order = sorted(range(len(candidates)), key=lambda i: candidates[i].lineno)
    for rank, i in enumerate(order):
        node = candidates[i]
        target = node.targets[0].id
        indent = re.match(r"[ \t]*", annotated_lines[node.lineno - 1 + inserted]).group(0)
        annotated_lines.insert(node.lineno + inserted, f"{indent}reveal_type({target})  # hyperray-candidate-{i}\n")
        inserted += 1

    probe_src = "".join(annotated_lines)

    with tempfile.NamedTemporaryFile("w", suffix=".py", delete=False) as f:
        f.write(probe_src)
        probe_path = f.name
    try:
        mypy_cmd = [python_executable or sys.executable, "-m", "mypy",
                    "--no-error-summary", "--hide-error-context", "--check-untyped-defs", probe_path]
        result = subprocess.run(mypy_cmd, capture_output=True, text=True, timeout=60)
    except Exception:
        os.unlink(probe_path)
        return src
    os.unlink(probe_path)

    resolved = {}  # candidate index -> revealed type string
    for out_line in result.stdout.splitlines():
        m = _REVEAL_RE.search(out_line)
        if not m:
            continue
        # the marker comment on the SAME probe line as the reveal_type call
        line_no = int(out_line.split(":", 2)[1]) if out_line.count(":") >= 2 else None
        if line_no is None or line_no - 1 >= len(annotated_lines):
            continue
        marker = re.search(r"# hyperray-candidate-(\d+)", annotated_lines[line_no - 1])
        if marker:
            resolved[int(marker.group(1))] = m.group(1)

    changed = False
    for i, node in enumerate(candidates):
        rtype = resolved.get(i)
        if rtype not in _SCALARS:
            continue
        scalar = rtype.rsplit(".", 1)[-1]
        new_node = ast.copy_location(
            ast.AnnAssign(target=node.targets[0], annotation=ast.Name(id=scalar, ctx=ast.Load()),
                          value=node.value, simple=1),
            node)
        ast.fix_missing_locations(new_node)
        _replace_stmt(mod, node, new_node)
        changed = True

    return ast.unparse(mod) if changed else src


def _replace_stmt(mod: ast.Module, old, new):
    for parent in ast.walk(mod):
        for field, value in ast.iter_fields(parent):
            if isinstance(value, list):
                for i, item in enumerate(value):
                    if item is old:
                        value[i] = new
                        return
