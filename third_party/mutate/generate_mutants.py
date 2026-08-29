#!/usr/bin/env python3
"""Generates every mutant of a source file, for hyperray's mutation pass:
does the task's own test suite actually verify each requirement, or
would it stay green while the behaviour changed?

A mutant is one small, deliberate change to the solution. Run the task's
tests against it:

  * tests fail  -> that behaviour IS verified; the mutant is "killed"
  * tests pass  -> the tests cannot tell the difference. If the mutant
                   genuinely behaves differently (hyperray checks this
                   separately, by running it), that is a PROVEN gap in
                   the test suite -- the exact false positive where an
                   agent meets requirements A and B, skips C, and still
                   passes.

Exhaustive, not sampled. The scope is one task's solution -- a few
hundred lines -- so every mutation point crossed with every operator is
a small, finite, fully enumerable set. That is what separates this from
asking a model to guess which tests look missing: the same input always
produces the same mutants, nothing is skipped, and a survivor is
evidence rather than a suggestion.

Mutation points come from each language's own tree-sitter grammar, so
adding a language means adding a table entry, not new traversal code.

Output JSON on stdout:
  {"path": "...", "language": "...", "mutants": [
     {"id": 0, "line": 12, "operator": "relational",
      "original": ">", "mutated": ">=", "source": "<full mutated file>"}]}
"""
import argparse
import json
import sys

from tree_sitter import Language, Parser, Query

try:
    from tree_sitter import QueryCursor
except ImportError:  # pragma: no cover - depends on binding version
    QueryCursor = None

# Replacements per operator class. Each entry maps a token to every other
# token it can become; applying all of them at every site is what makes
# the mutant set exhaustive rather than sampled.
_RELATIONAL = {
    ">": [">=", "<", "=="],
    ">=": [">", "<=", "=="],
    "<": ["<=", ">", "=="],
    "<=": ["<", ">=", "=="],
    "==": ["!="],
    "!=": ["=="],
}
_ARITHMETIC = {
    "+": ["-"],
    "-": ["+"],
    "*": ["/"],
    "/": ["*"],
    "%": ["*"],
}
_LOGICAL = {
    "and": ["or"],
    "or": ["and"],
    "&&": ["||"],
    "||": ["&&"],
}

_LANGUAGES = {
    "python": {
        "module": "tree_sitter_python",
        # Operator tokens live inside these nodes; capturing the node and
        # walking its direct children finds the token without hardcoding
        # a list of operator node types.
        "operator_nodes": "[(comparison_operator) (binary_operator) (boolean_operator)] @n",
        "number_nodes": "[(integer) (float)] @n",
        "bool_nodes": "[(true) (false)] @n",
    },
    "rust": {
        "module": "tree_sitter_rust",
        "operator_nodes": "(binary_expression) @n",
        "number_nodes": "[(integer_literal) (float_literal)] @n",
        "bool_nodes": "(boolean_literal) @n",
    },
    "cpp": {
        "module": "tree_sitter_cpp",
        "operator_nodes": "(binary_expression) @n",
        "number_nodes": "(number_literal) @n",
        "bool_nodes": "[(true) (false)] @n",
    },
    "go": {
        "module": "tree_sitter_go",
        "operator_nodes": "(binary_expression) @n",
        "number_nodes": "[(int_literal) (float_literal)] @n",
        "bool_nodes": "(true) @n",
    },
}


def _captured(query, tree, source):
    if QueryCursor is not None:
        caps = QueryCursor(query).captures(tree.root_node)
    else:
        caps = query.captures(tree.root_node)
    if isinstance(caps, dict):
        nodes = []
        for v in caps.values():
            nodes.extend(v)
        return nodes
    return [n for n, _ in caps]


def _splice(source, start, end, replacement):
    return source[:start] + replacement.encode() + source[end:]


def generate(path, language_name):
    spec = _LANGUAGES[language_name]
    lang = Language(__import__(spec["module"]).language())
    parser = Parser(lang)
    with open(path, "rb") as f:
        source = f.read()
    tree = parser.parse(source)

    mutants = []

    def add(node, original, mutated, operator):
        mutants.append({
            "line": node.start_point[0] + 1,
            "operator": operator,
            "original": original,
            "mutated": mutated,
            "source": _splice(source, node.start_byte, node.end_byte, mutated).decode(
                "utf-8", errors="replace"),
        })

    # Operator swaps: find the operator token among an expression's own
    # children rather than guessing its node type per language.
    for node in _captured(Query(lang, spec["operator_nodes"]), tree, source):
        for child in node.children:
            token = source[child.start_byte:child.end_byte].decode("utf-8", errors="replace")
            for table, opname in ((_RELATIONAL, "relational"),
                                  (_ARITHMETIC, "arithmetic"),
                                  (_LOGICAL, "logical")):
                if token in table:
                    for replacement in table[token]:
                        add(child, token, replacement, opname)
                    break

    # Boundary mutations: a constant n becomes n-1 and n+1. These are the
    # off-by-one errors, and the value that exposes each one is adjacent
    # to the constant already written in the source -- which is why hyperray
    # derives its boundary test inputs from these same constants rather
    # than hoping a harvested value lands on them.
    if "number_nodes" in spec:
        for node in _captured(Query(lang, spec["number_nodes"]), tree, source):
            raw = source[node.start_byte:node.end_byte].decode("utf-8", errors="replace")
            try:
                value = int(raw)
            except ValueError:
                continue
            add(node, raw, str(value + 1), "constant")
            add(node, raw, str(value - 1), "constant")

    if "bool_nodes" in spec:
        for node in _captured(Query(lang, spec["bool_nodes"]), tree, source):
            raw = source[node.start_byte:node.end_byte].decode("utf-8", errors="replace")
            flipped = {"True": "False", "False": "True",
                       "true": "false", "false": "true"}.get(raw)
            if flipped:
                add(node, raw, flipped, "boolean")

    mutants.sort(key=lambda m: (m["line"], m["operator"], m["mutated"]))
    for i, m in enumerate(mutants):
        m["id"] = i
    return mutants


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("source")
    ap.add_argument("language", choices=sorted(_LANGUAGES))
    ap.add_argument("--count-only", action="store_true",
                    help="report how many mutants exist without emitting their sources")
    args = ap.parse_args()

    mutants = generate(args.source, args.language)
    if args.count_only:
        for m in mutants:
            m.pop("source", None)
    json.dump({"path": args.source, "language": args.language, "mutants": mutants},
              sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
