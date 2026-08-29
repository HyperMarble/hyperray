"""Normalize a deliberately small, bounded subset of Python's AST.

The Go frontend embeds this file and executes it with the system Python.  This
script is not a verifier.  Its only job is to obtain Python's real parser and
return a lossless-enough, explicitly tagged syntax tree for the constructs the
Hyperray frontend supports.  Anything outside that allowlist is reported as an
unsupported node; callers must treat any such report as a proof blocker.
"""

import ast
import dis
import hashlib
import json
import symtable
import sys
import types


def location(node):
    return {
        "line": getattr(node, "lineno", 1),
        "column": getattr(node, "col_offset", 0) + 1,
        "end_line": getattr(node, "end_lineno", getattr(node, "lineno", 1)),
        # CPython's end offset is zero-based and exclusive. Numerically that is
        # the same as the one-based inclusive column required by Semantic IR.
        "end_column": getattr(node, "end_col_offset", getattr(node, "col_offset", 0) + 1),
    }


class Normalizer:
    SAFE_BUILTINS = {
        "abs", "all", "any", "bool", "enumerate", "float", "int", "isinstance",
        "len", "list", "max", "min", "range", "reversed", "round", "sorted",
        "str", "sum", "tuple", "zip",
    }
    DYNAMIC_CALLS = {
        "breakpoint", "compile", "delattr", "dir", "eval", "exec", "getattr",
        "globals", "hasattr", "help", "id", "input", "locals", "memoryview",
        "open", "setattr", "super", "type", "vars", "__import__",
    }
    SAFE_EXCEPTIONS = {
        "ArithmeticError", "AssertionError", "AttributeError", "BufferError",
        "EOFError", "Exception", "ImportError", "IndexError", "KeyError",
        "LookupError", "MemoryError", "NameError", "NotImplementedError",
        "OSError", "OverflowError", "ReferenceError", "RuntimeError",
        "StopIteration", "SyntaxError", "SystemError", "TypeError",
        "UnicodeError", "ValueError", "ZeroDivisionError",
    }
    EXPR_OPS = {
        ast.Add: "add", ast.Sub: "subtract", ast.Mult: "multiply", ast.Div: "divide",
        ast.FloorDiv: "floor_divide", ast.Mod: "modulo", ast.Pow: "power",
        ast.LShift: "shift_left", ast.RShift: "shift_right", ast.BitOr: "bit_or",
        ast.BitXor: "bit_xor", ast.BitAnd: "bit_and", ast.MatMult: "matrix_multiply",
        ast.And: "and", ast.Or: "or", ast.Not: "not", ast.UAdd: "positive",
        ast.USub: "negative", ast.Invert: "invert",
        ast.Eq: "equal", ast.NotEq: "not_equal", ast.Lt: "less_than",
        ast.LtE: "less_or_equal", ast.Gt: "greater_than", ast.GtE: "greater_or_equal",
        ast.Is: "is", ast.IsNot: "is_not", ast.In: "in", ast.NotIn: "not_in",
    }

    def __init__(self, source, entry_points, test_entry_points, symbols, module_exports, module_name="", future_annotations=False, exhaustive=False):
        self.source = source
        self.entry_points = set(entry_points)
        self.test_entry_points = set(test_entry_points or [])
        self.local_functions = set()
        self.imported_entries = set()
        self.imported_modules = set()
        self.import_aliases = {}
        self.imports = []
        self.unsupported = []
        self.bound_names = set()
        self.symbols = symbols
        self.module_exports = module_exports
        self.future_annotations = future_annotations
        self.module_name = module_name
        self.referenced_globals = set()
        self.exhaustive = exhaustive
        self.call_edges = []
        self.selected_entry_points = set()

    def reject(self, node, code, message):
        item = {"code": code, "message": message, "location": location(node)}
        if item not in self.unsupported:
            self.unsupported.append(item)

    def expression(self, node):
        base = {"location": location(node), "source": ast.get_source_segment(self.source, node) or ""}
        if isinstance(node, ast.Constant):
            value = node.value
            if not (value is None or isinstance(value, (bool, int, float, str))):
                self.reject(node, "PY_UNSUPPORTED_LITERAL", "unsupported literal value")
                return None
            return {**base, "kind": "literal", "value": value}
        if isinstance(node, ast.Name):
            if not isinstance(node.ctx, ast.Load):
                self.reject(node, "PY_UNSUPPORTED_NAME_CONTEXT", "only name reads are supported in expressions")
                return None
            return {**base, "kind": "name", "name": node.id}
        if isinstance(node, (ast.List, ast.Tuple, ast.Set)):
            elements = [self.expression(item) for item in node.elts]
            if any(item is None for item in elements):
                return None
            return {**base, "kind": type(node).__name__.lower(), "args": elements}
        if isinstance(node, ast.Dict):
            if any(key is None for key in node.keys):
                self.reject(node, "PY_UNSUPPORTED_DICT_EXPANSION", "dictionary expansion is not bounded")
                return None
            keys = [self.expression(item) for item in node.keys]
            values = [self.expression(item) for item in node.values]
            if any(item is None for item in keys + values):
                return None
            pairs = []
            for key, value in zip(keys, values):
                pairs.extend((key, value))
            return {**base, "kind": "dict", "args": pairs}
        if isinstance(node, ast.Attribute):
            if (
                node.attr == "__name__"
                and isinstance(node.value, ast.Call)
                and isinstance(node.value.func, ast.Name)
                and node.value.func.id == "type"
                and len(node.value.args) == 1
                and not node.value.keywords
            ):
                value = self.expression(node.value.args[0])
                if value is None:
                    return None
                return {**base, "kind": "call", "name": "__hyperray_type_name__", "args": [value]}
            if not self.exhaustive:
                self.reject(node, "PY_DYNAMIC_ATTRIBUTE", "attribute lookup can invoke descriptors and requires exact exhaustive CPython evidence")
                return None
            value = self.expression(node.value)
            if value is None:
                return None
            return {**base, "kind": "field", "name": node.attr, "args": [value]}
        if isinstance(node, ast.Subscript) and not isinstance(node.slice, ast.Slice):
            if not self.exhaustive:
                self.reject(node, "PY_DYNAMIC_SUBSCRIPT", "subscript lookup can invoke user code and requires exact exhaustive CPython evidence")
                return None
            value, index = self.expression(node.value), self.expression(node.slice)
            if value is None or index is None:
                return None
            return {**base, "kind": "index", "args": [value, index]}
        if isinstance(node, ast.JoinedStr):
            args = []
            for item in node.values:
                if isinstance(item, ast.Constant) and isinstance(item.value, str):
                    args.append({"kind": "literal", "value": item.value, "location": location(item), "source": ast.get_source_segment(self.source, item) or item.value})
                elif isinstance(item, ast.FormattedValue) and item.conversion in (-1, 115) and item.format_spec is None:
                    value = self.expression(item.value)
                    if value is None:
                        return None
                    args.append({"kind": "call", "name": "str", "args": [value], "location": location(item), "source": ast.get_source_segment(self.source, item) or ""})
                else:
                    self.reject(item, "PY_FORMAT_STRING", "format conversions/specifiers are outside bounded string semantics")
                    return None
            return {**base, "kind": "fstring", "args": args}
        if isinstance(node, ast.UnaryOp):
            operand = self.expression(node.operand)
            op = self.EXPR_OPS.get(type(node.op))
            if operand is None or op is None:
                self.reject(node, "PY_UNSUPPORTED_OPERATOR", "unsupported unary operator")
                return None
            return {**base, "kind": "unary", "operator": op, "args": [operand]}
        if isinstance(node, ast.BinOp):
            left, right = self.expression(node.left), self.expression(node.right)
            op = self.EXPR_OPS.get(type(node.op))
            if left is None or right is None or op is None:
                self.reject(node, "PY_UNSUPPORTED_OPERATOR", "unsupported binary operator")
                return None
            return {**base, "kind": "binary", "operator": op, "args": [left, right]}
        if isinstance(node, ast.BoolOp):
            args = [self.expression(item) for item in node.values]
            op = self.EXPR_OPS.get(type(node.op))
            if any(item is None for item in args) or op is None:
                self.reject(node, "PY_UNSUPPORTED_OPERATOR", "unsupported boolean operator")
                return None
            return {**base, "kind": "boolean", "operator": op, "args": args}
        if isinstance(node, ast.Compare):
            args = [self.expression(node.left)] + [self.expression(item) for item in node.comparators]
            ops = [self.EXPR_OPS.get(type(item)) for item in node.ops]
            if any(item is None for item in args) or any(item is None for item in ops):
                self.reject(node, "PY_UNSUPPORTED_COMPARISON", "unsupported comparison operator")
                return None
            return {**base, "kind": "comparison", "operators": ops, "args": args}
        if isinstance(node, ast.IfExp):
            args = [self.expression(node.test), self.expression(node.body), self.expression(node.orelse)]
            if any(item is None for item in args):
                return None
            return {**base, "kind": "conditional", "args": args}
        if isinstance(node, ast.Call):
            return self.call(node)
        # Attribute and subscript access can invoke arbitrary user code in Python,
        # so neither is a safe syntactic approximation of a field/array read.
        self.reject(node, "PY_UNSUPPORTED_EXPRESSION", "unsupported expression: " + type(node).__name__)
        return None

    def call_target(self, node):
        if isinstance(node, ast.Name):
            canonical = self.import_aliases.get(node.id, node.id)
            return canonical, canonical
        if isinstance(node, ast.Attribute) and isinstance(node.value, ast.Name):
            if node.value.id in self.imported_modules:
                return node.attr, node.attr
            return node.value.id + "." + node.attr, node.attr
        self.reject(node, "PY_DYNAMIC_CALL", "call target is computed dynamically")
        return None, None

    def call(self, node, exception_constructor=False):
        base = {"location": location(node), "source": ast.get_source_segment(self.source, node) or ""}
        qualified, simple = self.call_target(node.func)
        if qualified is None:
            return None
        if simple in self.DYNAMIC_CALLS or qualified.startswith(("inspect.", "importlib.", "builtins.")):
            self.reject(node, "PY_DYNAMIC_CALL", "dynamic or reflective call is unsupported: " + qualified)
        if simple in self.bound_names:
            self.reject(node, "PY_DYNAMIC_CALL", "call target is shadowed by a local parameter: " + simple)
        allowed = simple in self.local_functions or simple in self.entry_points or simple in self.imported_entries
        allowed = allowed or simple in self.SAFE_BUILTINS
        if simple in self.bound_names:
            allowed = False
        if exception_constructor:
            allowed = simple in self.SAFE_EXCEPTIONS and simple not in self.local_functions and simple not in self.bound_names
        if "." in qualified:
            module = qualified.split(".", 1)[0]
            allowed = allowed and module in self.imported_modules and simple in self.entry_points
        if not allowed:
            self.reject(node, "PY_EXTERNAL_CALL", "unresolved or external call is unsupported: " + qualified)
        if node.keywords:
            self.reject(node, "PY_KEYWORD_CALL", "keyword arguments are not supported by bounded call semantics")
        args = [self.expression(item) for item in node.args]
        if any(item is None for item in args):
            return None
        return {**base, "kind": "call", "name": qualified, "args": args}

    def statement(self, node, in_test):
        base = {"location": location(node), "source": ast.get_source_segment(self.source, node) or ""}
        if isinstance(node, ast.Return):
            value = self.expression(node.value) if node.value is not None else None
            if node.value is not None and value is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "return", "value": value}
        if isinstance(node, ast.Raise):
            if node.cause is not None:
                self.reject(node.cause, "PY_RAISE_CAUSE", "raise-from cause semantics are unsupported")
            if node.exc is None:
                self.reject(node, "PY_RERAISE", "bare re-raise requires dynamic exception state")
                return {**base, "kind": "raise"}
            if isinstance(node.exc, ast.Call):
                qualified, simple = self.call_target(node.exc.func)
                if qualified is None or simple not in self.SAFE_EXCEPTIONS or simple in self.local_functions or simple in self.bound_names:
                    self.reject(node.exc, "PY_DYNAMIC_RAISE", "raised exception must be an unshadowed built-in exception")
                if len(node.exc.args) > 1:
                    self.reject(node.exc, "PY_DYNAMIC_EXCEPTION_MESSAGE", "exception constructor accepts at most one exactly translated expression")
                if node.exc.args and not (isinstance(node.exc.args[0], ast.Constant) and isinstance(node.exc.args[0].value, str)) and not self.exhaustive:
                    self.reject(node.exc, "PY_DYNAMIC_EXCEPTION_MESSAGE", "computed exception messages require exact exhaustive CPython evidence")
                value = self.call(node.exc, exception_constructor=True)
            elif isinstance(node.exc, ast.Name) and node.exc.id in self.SAFE_EXCEPTIONS and node.exc.id not in self.local_functions and node.exc.id not in self.bound_names:
                value = self.expression(node.exc)
            else:
                self.reject(node.exc, "PY_DYNAMIC_RAISE", "raised exception must be an unshadowed built-in exception")
                value = self.expression(node.exc)
            if value is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "raise", "value": value}
        if isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
            value = self.call(node.value)
            if value is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "call", "value": value}
        if isinstance(node, ast.Assert):
            if not in_test:
                self.reject(node, "PY_RUNTIME_ASSERT", "runtime assert in solution code is not a test observation")
            predicate = self.expression(node.test)
            if node.msg is not None:
                self.reject(node.msg, "PY_ASSERT_MESSAGE", "assert messages are not modeled")
            if predicate is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "assert", "value": predicate}
        if isinstance(node, ast.With) and in_test:
            if len(node.items) != 1 or node.items[0].optional_vars is not None or len(node.body) != 1:
                self.reject(node, "PY_UNSUPPORTED_WITH", "only one exact pytest.raises observation is supported")
                return {**base, "kind": "unsupported"}
            context = node.items[0].context_expr
            is_raises = (
                isinstance(context, ast.Call)
                and isinstance(context.func, ast.Attribute)
                and isinstance(context.func.value, ast.Name)
                and context.func.value.id == "pytest"
                and context.func.attr == "raises"
            )
            observed = node.body[0]
            if not is_raises or not isinstance(observed, ast.Expr) or not isinstance(observed.value, ast.Call):
                self.reject(node, "PY_UNSUPPORTED_WITH", "with statement is not an exact pytest.raises call observation")
                return {**base, "kind": "unsupported"}
            if len(context.args) != 1 or not isinstance(context.args[0], ast.Name):
                self.reject(context, "PY_DYNAMIC_EXCEPTION_ASSERT", "pytest.raises exception type must be a static name")
                return {**base, "kind": "unsupported"}
            if context.keywords:
                self.reject(context, "PY_EXCEPTION_REGEX", "pytest.raises match uses regex semantics absent from Test IR")
                return {**base, "kind": "unsupported"}
            observed_call = self.call(observed.value)
            if observed_call is None:
                return {**base, "kind": "unsupported"}
            return {
                **base,
                "kind": "assert_raises",
                "value": observed_call,
                "exception_type": context.args[0].id,
                "message": "",
            }
        if isinstance(node, ast.If):
            condition = self.expression(node.test)
            body = [self.statement(item, in_test) for item in node.body]
            alternate = [self.statement(item, in_test) for item in node.orelse]
            if condition is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "branch", "value": condition, "body": body, "alternate": alternate}
        if isinstance(node, ast.Assign):
            if len(node.targets) != 1 or not isinstance(node.targets[0], ast.Name):
                self.reject(node, "PY_DYNAMIC_ASSIGNMENT", "only one local-name assignment is supported")
                return {**base, "kind": "unsupported"}
            value = self.expression(node.value)
            if value is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "assign", "target": node.targets[0].id, "value": value}
        if isinstance(node, ast.For):
            if not isinstance(node.target, ast.Name) or node.orelse or getattr(node, "type_comment", None):
                self.reject(node, "PY_DYNAMIC_LOOP", "loop requires a local-name target, finite iterator, and no else/type comment")
                return {**base, "kind": "unsupported"}
            iterator = self.expression(node.iter)
            if iterator is None:
                return {**base, "kind": "unsupported"}
            return {**base, "kind": "loop", "target": node.target.id, "value": iterator, "body": [self.statement(item, in_test) for item in node.body]}
        if isinstance(node, ast.Try):
            if node.orelse or node.finalbody or not node.handlers:
                self.reject(node, "PY_DYNAMIC_TRY", "try requires typed except handlers and no else/finally")
                return {**base, "kind": "unsupported"}
            catches = []
            for handler in node.handlers:
                if handler.name is not None or not isinstance(handler.type, ast.Name) or handler.type.id not in self.SAFE_EXCEPTIONS:
                    self.reject(handler, "PY_DYNAMIC_EXCEPT", "except handler must name one unbound built-in exception without alias")
                    return {**base, "kind": "unsupported"}
                catches.append({"exception_type": handler.type.id, "location": location(handler), "body": [self.statement(item, in_test) for item in handler.body]})
            return {**base, "kind": "try", "body": [self.statement(item, in_test) for item in node.body], "catches": catches}
        if isinstance(node, ast.Continue):
            return {**base, "kind": "continue"}
        if isinstance(node, ast.Pass):
            return {**base, "kind": "pass"}
        if isinstance(node, ast.FunctionDef):
            self.reject(node, "PY_NESTED_FUNCTION", "nested function definitions have dynamic closure semantics")
            return {**base, "kind": "unsupported"}
        self.reject(node, "PY_UNSUPPORTED_STATEMENT", "unsupported statement: " + type(node).__name__)
        return {**base, "kind": "unsupported"}

    def controlled_import(self, node):
        if (
            isinstance(node, ast.ImportFrom)
            and node.module == "__future__"
            and node.level == 0
            and len(node.names) == 1
            and node.names[0].name == "annotations"
            and node.names[0].asname is None
        ):
            return
        local_import_names = []
        for alias in node.names:
            local_import_names.append(alias.asname or (alias.name.split(".", 1)[0] if isinstance(node, ast.Import) else alias.name))
        if not any(name in self.referenced_globals for name in local_import_names):
            return
        if (
            getattr(self, "artifact_kind", "") == "tests"
            and isinstance(node, ast.Import)
            and len(node.names) == 1
            and node.names[0].name == "pytest"
            and (node.names[0].asname in (None, "pytest"))
        ):
            self.reject(node, "PY_TEST_FRAMEWORK_IMPORT", "exact test observation projection does not model pytest module import semantics")
            return
        if isinstance(node, ast.ImportFrom):
            resolved_module = node.module or ""
            if node.level:
                package = self.module_name.rsplit(".", 1)[0] if "." in self.module_name else self.module_name
                parts = package.split(".") if package else []
                remove = node.level - 1
                if remove > len(parts):
                    self.reject(node, "PY_UNRESOLVED_IMPORT", "relative import escapes the frozen package")
                    return
                parts = parts[:len(parts) - remove] if remove else parts
                resolved_module = ".".join(parts + ([node.module] if node.module else []))
            exports = self.module_exports.get(resolved_module)
            if exports is None:
                self.reject(node, "PY_UNRESOLVED_IMPORT", "imported module is not a frozen Python focus artifact")
                return
            names = []
            for alias in node.names:
                if alias.name == "*" or alias.name not in exports or (not self.exhaustive and alias.name not in self.entry_points):
                    self.reject(node, "PY_EXTERNAL_IMPORT", "import must name a frozen export in the declared bounded operation scope")
                    return
                names.append(alias.asname or alias.name)
                self.import_aliases[alias.asname or alias.name] = alias.name
            self.imported_entries.update(names)
            self.imports.append({"module": resolved_module, "names": names, "location": location(node)})
            return
        if isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name not in self.module_exports:
                    self.reject(node, "PY_EXTERNAL_IMPORT", "module is not a frozen Python focus artifact: " + alias.name)
                    continue
                self.imported_modules.add(alias.asname or alias.name)
                self.imports.append({"module": alias.name, "names": [], "location": location(node)})
            return
        self.reject(node, "PY_EXTERNAL_IMPORT", "relative imports are unsupported")

    def function(self, node, artifact_kind):
        if node.decorator_list:
            self.reject(node, "PY_DECORATOR", "decorated functions require unmodeled call semantics")
        annotations = [item.annotation for item in node.args.posonlyargs + node.args.args + node.args.kwonlyargs]
        if not self.future_annotations and (node.returns is not None or node.type_comment is not None or any(item is not None for item in annotations)):
            self.reject(node, "PY_ANNOTATION", "annotations can execute names/calls and are not runtime type guarantees")
        args = node.args
        if args.vararg or args.kwarg or args.kwonlyargs or args.defaults or args.kw_defaults:
            self.reject(node, "PY_DYNAMIC_SIGNATURE", "variadic, keyword-only, or default parameters are unsupported")
        parameters = [item.arg for item in args.posonlyargs + args.args]
        symbol_table = self.symbols.get((node.name, node.lineno))
        if symbol_table is None:
            self.reject(node, "PY_SYMBOL_TABLE", "function has no matching standard-library symtable scope")
        else:
            for symbol in symbol_table.get_symbols():
                if symbol.is_free() or symbol.is_nonlocal():
                    self.reject(node, "PY_CLOSURE_BINDING", "free/nonlocal bindings require dynamic closure semantics: " + symbol.get_name())
                if (
                    symbol.is_global()
                    and symbol.is_referenced()
                    and symbol.get_name() not in self.local_functions
                    and symbol.get_name() not in self.imported_entries
                    and symbol.get_name() not in self.imported_modules
                    and symbol.get_name() not in self.SAFE_BUILTINS
                    and symbol.get_name() not in self.SAFE_EXCEPTIONS
                ):
                    self.reject(node, "PY_GLOBAL_STATE", "function reads unmodeled module/global state: " + symbol.get_name())
        in_test = artifact_kind == "tests" or node.name.startswith("test_")
        previous_bound = self.bound_names
        self.bound_names = set(parameters)
        body = []
        for index, item in enumerate(node.body):
            if index == 0 and isinstance(item, ast.Expr) and isinstance(item.value, ast.Constant) and isinstance(item.value.value, str):
                body.append({**{"location": location(item), "source": ast.get_source_segment(self.source, item) or ""}, "kind": "docstring"})
            else:
                body.append(self.statement(item, in_test))
        self.bound_names = previous_bound
        return {
            "name": node.name,
            "parameters": parameters,
            "is_test": in_test,
            "is_entry_point": node.name in self.selected_entry_points,
            "location": location(node),
            "body": body,
        }

    def module(self, tree, artifact_kind):
        self.artifact_kind = artifact_kind
        self.local_functions = {node.name for node in tree.body if isinstance(node, ast.FunctionDef)}
        function_nodes = {
            node.name: node for node in tree.body if isinstance(node, ast.FunctionDef)
        }
        direct_local_calls = {name: [] for name in function_nodes}
        for caller, function_node in function_nodes.items():
            for child in ast.walk(function_node):
                if (
                    isinstance(child, ast.Call)
                    and isinstance(child.func, ast.Name)
                    and child.func.id in function_nodes
                ):
                    direct_local_calls[caller].append((child.func.id, child))

        if artifact_kind == "tests":
            roots = {
                node.name for node in tree.body
                if isinstance(node, ast.FunctionDef)
                and node.name.startswith("test_")
                and (not self.test_entry_points or node.name in self.test_entry_points)
            }
            reachable = set(roots)
        else:
            roots = set(self.entry_points) if self.entry_points else set(function_nodes)
            reachable = set()
            pending = list(roots)
            while pending:
                name = pending.pop()
                if name in reachable or name not in function_nodes:
                    continue
                reachable.add(name)
                pending.extend(callee for callee, _ in direct_local_calls[name])
        self.selected_entry_points = roots

        def selected(node):
            if artifact_kind == "tests":
                return node.name.startswith("test_") and (not self.test_entry_points or node.name in self.test_entry_points)
            return node.name in reachable

        for node in tree.body:
            if not isinstance(node, ast.FunctionDef) or not selected(node):
                continue
            table = self.symbols.get((node.name, node.lineno))
            if table is not None:
                self.referenced_globals.update(symbol.get_name() for symbol in table.get_symbols() if symbol.is_global() and symbol.is_referenced())
        for node in tree.body:
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                self.controlled_import(node)
        functions = []
        seen_functions = set()
        for node in tree.body:
            if isinstance(node, ast.FunctionDef) and selected(node):
                if node.name in seen_functions:
                    self.reject(node, "PY_DUPLICATE_FUNCTION", "duplicate function definition changes the callable binding: " + node.name)
                seen_functions.add(node.name)
                functions.append(self.function(node, artifact_kind))
            elif isinstance(node, ast.FunctionDef):
                if artifact_kind == "tests":
                    args = node.args
                    annotations = [item.annotation for item in args.posonlyargs + args.args + args.kwonlyargs]
                    if (
                        node.decorator_list
                        or args.defaults
                        or any(item is not None for item in args.kw_defaults)
                        or node.returns is not None
                        or node.type_comment is not None
                        or any(item is not None for item in annotations)
                    ):
                        self.reject(node, "PY_TEST_IMPORT_TIME", "unselected helper definition executes decorator/default/annotation semantics at module import")
                continue
            elif isinstance(node, (ast.ClassDef, ast.Assign, ast.AnnAssign)):
                if artifact_kind == "tests":
                    self.reject(node, "PY_TEST_IMPORT_TIME", "class or assignment executes unmodeled state at test-module import")
                continue
            elif isinstance(node, (ast.Import, ast.ImportFrom)):
                continue
            elif isinstance(node, ast.Expr) and isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
                continue
            elif self.entry_points:
                # EntryPoints is an explicit proof-scope cut. Top-level setup
                # outside the selected callable is workspace evidence, not an
                # operation body; concrete execution imports the frozen target
                # module independently.
                continue
            else:
                self.reject(node, "PY_UNSUPPORTED_TOP_LEVEL", "unsupported top-level construct: " + type(node).__name__)
        declarations = [
            {"name": node.name, "location": location(node)}
            for node in tree.body
            if isinstance(node, ast.FunctionDef) and node.name in reachable
        ]
        for caller in sorted(reachable):
            for callee, call_node in direct_local_calls.get(caller, []):
                if callee in reachable:
                    self.call_edges.append({
                        "caller": caller,
                        "callee": callee,
                        "location": location(call_node),
                    })
        return {
            "functions": functions,
            "module_declarations": declarations,
            "call_edges": self.call_edges,
            "imports": self.imports,
            "unsupported": self.unsupported,
        }


def main():
    request = json.load(sys.stdin)
    source = request.get("source", "")
    entry_points = request.get("entry_points", [])
    test_entry_points = request.get("test_entry_points", [])
    resolved_modules = request.get("resolved_modules", {})
    artifact_kind = request.get("artifact_kind", "solution")
    module_name = request.get("module_name", "")
    exhaustive = request.get("execution") in ("exhaustive", "bound-cpython")
    try:
        tree = ast.parse(source, filename=request.get("path") or "<artifact>", type_comments=True)
        root_symbols = symtable.symtable(source, request.get("path") or "<artifact>", "exec")
    except (SyntaxError, ValueError, TypeError) as exc:
        loc = {
            "line": getattr(exc, "lineno", 1) or 1,
            "column": getattr(exc, "offset", 1) or 1,
            "end_line": getattr(exc, "end_lineno", getattr(exc, "lineno", 1)) or 1,
            "end_column": getattr(exc, "end_offset", getattr(exc, "offset", 1)) or 1,
        }
        json.dump({"parse_error": str(exc), "location": loc}, sys.stdout)
        return
    symbols = {(child.get_name(), child.get_lineno()): child for child in root_symbols.get_children()}
    module_exports = {}
    dependency_trees = {}
    for module_name, module_source in resolved_modules.items():
        try:
            module_tree = ast.parse(module_source, filename=module_name, type_comments=True)
            symtable.symtable(module_source, module_name, "exec")
        except (SyntaxError, ValueError, TypeError) as exc:
            json.dump({"parse_error": "frozen import " + module_name + ": " + str(exc), "location": {"line": 1, "column": 1, "end_line": 1, "end_column": 1}}, sys.stdout)
            return
        exports = {item.name for item in module_tree.body if isinstance(item, (ast.FunctionDef, ast.ClassDef))}
        for item in module_tree.body:
            if isinstance(item, ast.ImportFrom):
                exports.update(alias.asname or alias.name for alias in item.names if alias.name != "*")
        module_exports[module_name] = exports
        dependency_trees[module_name] = module_tree
    future_annotations = any(
        isinstance(item, ast.ImportFrom)
        and item.module == "__future__"
        and any(alias.name == "annotations" for alias in item.names)
        for item in tree.body
    )
    normalizer = Normalizer(source, entry_points, test_entry_points, symbols, module_exports, module_name, future_annotations, exhaustive)
    result = normalizer.module(tree, artifact_kind)
    if artifact_kind == "tests":
        compiled = compile(source, request.get("path") or "<artifact>", "exec", dont_inherit=True)
        code_objects = {
            item.co_name: item
            for item in compiled.co_consts
            if isinstance(item, types.CodeType)
        }
        selected_tests = {
            item["name"] for item in result["functions"] if item.get("is_test")
        }
        compiler_ir = {}
        compiler_ir["<module>"] = [
            {
                "id": "<module>:" + str(item.offset),
                "offset": item.offset,
                "opcode": item.opname,
                "argument": item.argrepr,
                "line": int(item.positions.lineno or item.starts_line or 1),
                "end_line": int(item.positions.end_lineno or item.positions.lineno or 1),
            }
            for item in dis.get_instructions(compiled, adaptive=False)
        ]
        for test_id in sorted(selected_tests):
            code = code_objects.get(test_id)
            if code is None:
                result["unsupported"].append({
                    "code": "PY_TEST_BYTECODE",
                    "message": "selected test has no compiled code object: " + test_id,
                    "location": {"line": 1, "column": 1, "end_line": 1, "end_column": 1},
                })
                continue
            compiler_ir[test_id] = [
                {
                    "id": test_id + ":" + str(item.offset),
                    "offset": item.offset,
                    "opcode": item.opname,
                    "argument": item.argrepr,
                    "line": int(item.positions.lineno or item.starts_line or code.co_firstlineno),
                    "end_line": int(item.positions.end_lineno or item.positions.lineno or code.co_firstlineno),
                }
                for item in dis.get_instructions(code, adaptive=False)
            ]
        compiler_bytes = json.dumps(compiler_ir, sort_keys=True, separators=(",", ":")).encode()
        result["compiler_ir"] = compiler_ir
        result["compiler_ir_digest"] = "sha256:" + hashlib.sha256(compiler_bytes).hexdigest()
    if exhaustive and artifact_kind != "tests":
        forbidden_modules = {"ctypes", "http", "importlib", "multiprocessing", "os", "pathlib", "random", "secrets", "socket", "subprocess", "tempfile", "time", "urllib", "uuid"}
        forbidden_calls = {"__import__", "breakpoint", "compile", "eval", "exec", "globals", "input", "locals", "open", "vars"}
        for dependency_name, dependency_tree in {"<focused>": tree, **dependency_trees}.items():
            for node in ast.walk(dependency_tree):
                imported_root = None
                if isinstance(node, ast.Import):
                    roots = {alias.name.split(".", 1)[0] for alias in node.names}
                    imported_root = next((root for root in roots if root in forbidden_modules), None)
                elif isinstance(node, ast.ImportFrom) and node.module:
                    root = node.module.split(".", 1)[0]
                    imported_root = root if root in forbidden_modules else None
                if imported_root:
                    result["unsupported"].append({"code": "PY_UNCONTROLLED_DEPENDENCY", "message": "frozen module " + dependency_name + " imports uncontrolled runtime module " + imported_root, "location": location(node)})
                if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in forbidden_calls:
                    result["unsupported"].append({"code": "PY_DYNAMIC_DEPENDENCY", "message": "frozen module " + dependency_name + " contains dynamic call " + node.func.id, "location": location(node)})
    json.dump(result, sys.stdout, separators=(",", ":"))


if __name__ == "__main__":
    main()
