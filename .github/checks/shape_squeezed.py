# Purpose: name Rust code squeezed onto fewer lines instead of written to be
#          read: a closure kept in a variable, a closure inside a closure,
#          a loop written as a method call, and a long chain of method calls.
# Never:   decide a limit here; every number comes from shape_rules.
"""Each check returns the problem as text, or None when the rule holds."""

import shape_rules as rules
from tree_sitter import Node

# A `?` or `.await` sits between two calls of a chain without ending it.
PASS_THROUGH = frozenset({"try_expression", "await_expression"})


def method_called(node: Node) -> str | None:
    """The name of the method `node` calls, or None when it calls no method."""
    if node.type != "call_expression":
        return None
    callee = node.child_by_field_name("function")
    if callee is None or callee.type != "field_expression":
        return None
    field = callee.child_by_field_name("field")
    if field is None or field.text is None:
        return None
    return field.text.decode()


def called_on(call: Node) -> Node | None:
    """What a method call is called on, looking through `?` and `.await`."""
    callee = call.child_by_field_name("function")
    target = callee.child_by_field_name("value") if callee else None
    while target is not None and target.type in PASS_THROUGH:
        target = target.children[0]
    return target


def chain_length(call: Node) -> int:
    """How many method calls follow one another, counting back from `call`."""
    length = 0
    current: Node | None = call
    while current is not None and method_called(current) is not None:
        length += 1
        current = called_on(current)
    return length


def is_last_in_chain(call: Node) -> bool:
    """True when no further method is called on the result of `call`."""
    above = call.parent
    while above is not None and above.type in PASS_THROUGH:
        above = above.parent
    if above is None or above.type != "field_expression":
        return True
    user = above.parent
    return user is None or user.child_by_field_name("function") != above


def is_inside_closure(node: Node) -> bool:
    """True when a closure encloses `node` within the same function."""
    above = node.parent
    while above is not None and above.type != "function_item":
        if above.type == "closure_expression":
            return True
        above = above.parent
    return False


def closure_problem(node: Node) -> str | None:
    """A closure kept in a variable, or a closure inside another closure."""
    if node.type != "closure_expression":
        return None
    holder = node.parent
    if holder is not None and holder.type == "let_declaration":
        return "closure kept in a variable; write a named function"
    if is_inside_closure(node):
        return "closure inside a closure; write a named function"
    return None


def call_problem(node: Node) -> str | None:
    """A loop written as a method call, or a chain longer than the limit."""
    method = method_called(node)
    if method is None:
        return None
    if method in rules.LOOP_METHODS:
        return f"`.{method}()` is a loop; write a `for` loop"
    length = chain_length(node)
    if is_last_in_chain(node) and length > rules.MAX_CHAIN_CALLS:
        return f"chain of {length} method calls (max {rules.MAX_CHAIN_CALLS})"
    return None


def squeezed_rust(node: Node) -> str | None:
    """Names the squeezed construct `node` is, if it is one."""
    return closure_problem(node) or call_problem(node)
