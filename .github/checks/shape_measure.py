# Purpose: measure one parsed file against the shape rules, as messages.
# Never:   decide a limit here; every number comes from shape_rules.
"""Each function returns problems as text; an empty list means the rule holds."""

import shape_rules as rules
from shape_rules import Language
from tree_sitter import Node


def code_lines(text: str, language: Language) -> int:
    """Lines that are neither blank nor only a comment."""
    stripped = (line.strip() for line in text.splitlines())
    kept = [line for line in stripped if line]
    return sum(not line.startswith(language.comment_prefixes) for line in kept)


def is_else_if(node: Node) -> bool:
    """An `else if` continues its `if`, so it is not one level deeper."""
    parent = node.parent
    if parent is None:
        return False
    if parent.type == "else_clause":
        return True
    return parent.child_by_field_name("alternative") == node


def nesting(node: Node, language: Language) -> int:
    """Deepest chain of nesting blocks below `node`, skipping inner functions."""
    deepest = 0
    for child in node.children:
        if child.type in language.functions:
            continue
        opens = child.type in language.nesting and not is_else_if(child)
        deepest = max(deepest, nesting(child, language) + int(opens))
    return deepest


def function_problems(function: Node, language: Language) -> list[str]:
    """Length and nesting problems of one function."""
    name_node = function.child_by_field_name("name")
    name = name_node.text.decode() if name_node and name_node.text else "?"
    where = f"{function.start_point.row + 1}: function `{name}`"
    problems = []
    length = function.end_point.row - function.start_point.row + 1
    if length > rules.MAX_FUNCTION_LINES:
        problems.append(f"{where} is {length} lines (max {rules.MAX_FUNCTION_LINES})")
    depth = nesting(function, language)
    if depth > rules.MAX_NESTING:
        problems.append(f"{where} nests {depth} deep (max {rules.MAX_NESTING})")
    return problems


def forbidden_rust(node: Node) -> str | None:
    """Names a construct that hides a failure instead of returning it."""
    if node.type == "call_expression":
        callee = node.child_by_field_name("function")
        field = callee.child_by_field_name("field") if callee else None
        if field and field.text and field.text.decode() in rules.FORBIDDEN_RUST_METHODS:
            return f"`.{field.text.decode()}()`"
    if node.type == "macro_invocation":
        macro = node.child_by_field_name("macro")
        if macro and macro.text and macro.text.decode() in rules.FORBIDDEN_RUST_MACROS:
            return f"`{macro.text.decode()}!`"
    pattern = node.child_by_field_name("pattern")
    if node.type == "let_declaration" and pattern and pattern.type == "_":
        return "`let _ =`"
    return None
