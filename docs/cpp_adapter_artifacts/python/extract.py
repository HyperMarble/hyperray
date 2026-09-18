#!/usr/bin/env python3
"""extract.py — Hyperray stage-1 EXTRACT for C++ via libclang Python bindings.

Emits: types, function signatures, constants inside function bodies,
branch points (with arm lists), loops, and arithmetic obligation sites.
"""
import sys, json, clang.cindex as ci

ci.Config.set_library_file("/Library/Developer/CommandLineTools/usr/lib/libclang.dylib")

SRC = sys.argv[1]
ARGS = ["-std=c++17", "-x", "c++"] + sys.argv[2:]

idx = ci.Index.create()
tu = idx.parse(SRC, args=ARGS)
for d in tu.diagnostics:
    if d.severity >= 3:
        print("DIAG:", d.spelling, file=sys.stderr)

def in_main(c):
    return c.location.file is not None and c.location.file.name == SRC

out = {"file": SRC, "records": [], "enums": [], "functions": [], "globals": []}

def sig(c):
    params = [{"name": a.spelling, "type": a.type.spelling} for a in c.get_arguments()]
    tparams = []
    if c.kind == ci.CursorKind.FUNCTION_TEMPLATE:
        # get_arguments() returns nothing for a FUNCTION_TEMPLATE; walk children
        for ch in c.get_children():
            if ch.kind == ci.CursorKind.PARM_DECL:
                params.append({"name": ch.spelling, "type": ch.type.spelling})
            elif ch.kind in (ci.CursorKind.TEMPLATE_TYPE_PARAMETER,
                             ci.CursorKind.TEMPLATE_NON_TYPE_PARAMETER):
                tparams.append(ch.spelling)
    return {
        "name": c.spelling,
        "mangled": c.mangled_name,
        "qualified": c.type.spelling,
        "result": c.result_type.spelling,
        "params": params,
        "template_params": tparams,
        "is_template": c.kind == ci.CursorKind.FUNCTION_TEMPLATE,
        "line": c.location.line,
    }

def walk_body(c):
    """constants, branch points, loops, arithmetic sites inside one body"""
    facts = {"int_literals": [], "branches": [], "loops": [], "arith": [],
             "subscripts": 0, "calls": []}
    def rec(n, depth=0):
        # STL header internals leak into the subtree; count only main-file nodes
        if n.location.file is not None and n.location.file.name != SRC:
            return
        k = n.kind
        if k == ci.CursorKind.INTEGER_LITERAL:
            toks = [t.spelling for t in n.get_tokens()]
            if toks:
                facts["int_literals"].append(toks[0])
        elif k == ci.CursorKind.IF_STMT:
            facts["branches"].append({"kind": "if", "line": n.location.line, "arms": 2})
        elif k == ci.CursorKind.SWITCH_STMT:
            arms = sum(1 for ch in n.walk_preorder()
                       if ch.kind in (ci.CursorKind.CASE_STMT, ci.CursorKind.DEFAULT_STMT))
            facts["branches"].append({"kind": "switch", "line": n.location.line, "arms": arms})
        elif k == ci.CursorKind.CONDITIONAL_OPERATOR:
            facts["branches"].append({"kind": "ternary", "line": n.location.line, "arms": 2})
        elif k in (ci.CursorKind.WHILE_STMT, ci.CursorKind.FOR_STMT,
                   ci.CursorKind.DO_STMT, ci.CursorKind.CXX_FOR_RANGE_STMT):
            facts["loops"].append({"kind": str(k).split(".")[-1], "line": n.location.line})
        elif k == ci.CursorKind.BINARY_OPERATOR:
            toks = [t.spelling for t in n.get_tokens()]
            # libclang <16 has no .opcode; recover operator from token stream
            op = None
            for t in toks:
                if t in ("+", "-", "*", "/", "%", "<<", ">>"):
                    op = t; break
            if op:
                facts["arith"].append({"op": op, "line": n.location.line,
                                       "type": n.type.spelling})
        elif k == ci.CursorKind.ARRAY_SUBSCRIPT_EXPR:
            facts["subscripts"] += 1
        elif k == ci.CursorKind.CALL_EXPR:
            # libclang's Python binding has NO CXX_OPERATOR_CALL_EXPR kind:
            # `v[i]` on a class type arrives as CALL_EXPR spelled "operator[]".
            if n.spelling == "operator[]":
                facts["subscripts"] += 1
            elif n.spelling:
                facts["calls"].append(n.spelling)
        for ch in n.get_children():
            rec(ch, depth + 1)
    rec(c)
    return facts

def visit(c):
    if c.kind != ci.CursorKind.TRANSLATION_UNIT and not in_main(c):
        return
    k = c.kind
    if k in (ci.CursorKind.STRUCT_DECL, ci.CursorKind.CLASS_DECL) and c.is_definition():
        out["records"].append({
            "name": c.spelling, "line": c.location.line,
            "fields": [{"name": f.spelling, "type": f.type.spelling}
                       for f in c.type.get_fields()],
            "methods": [m.spelling for m in c.get_children()
                        if m.kind in (ci.CursorKind.CXX_METHOD,
                                      ci.CursorKind.CONSTRUCTOR)],
        })
    elif k == ci.CursorKind.ENUM_DECL and c.is_definition():
        out["enums"].append({
            "name": c.spelling, "line": c.location.line,
            "scoped": c.is_scoped_enum() if hasattr(c, "is_scoped_enum") else None,
            "variants": [{"name": e.spelling, "value": e.enum_value}
                         for e in c.get_children()
                         if e.kind == ci.CursorKind.ENUM_CONSTANT_DECL],
        })
    elif k in (ci.CursorKind.FUNCTION_DECL, ci.CursorKind.CXX_METHOD,
               ci.CursorKind.FUNCTION_TEMPLATE, ci.CursorKind.CONSTRUCTOR):
        if c.is_definition():
            s = sig(c)
            s["body"] = walk_body(c)
            out["functions"].append(s)
    elif k == ci.CursorKind.VAR_DECL and c.semantic_parent.kind in (
            ci.CursorKind.TRANSLATION_UNIT, ci.CursorKind.NAMESPACE):
        toks = [t.spelling for t in c.get_tokens()]
        out["globals"].append({"name": c.spelling, "type": c.type.spelling,
                               "tokens": toks, "line": c.location.line})
    for ch in c.get_children():
        visit(ch)

visit(tu.cursor)
json.dump(out, sys.stdout, indent=1)
