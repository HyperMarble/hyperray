# Purpose: check staged Rust, Python and Go files against the repo's shape rules.
# Never:   pass a file it could not read or parse; that is reported, not skipped.
"""Usage: shape.py FILE... Prints each problem and exits 1 if there is any."""

import sys
from pathlib import Path

import shape_rules as rules
from shape_measure import code_lines, forbidden_rust, function_problems
from shape_rules import Language
from tree_sitter import Language as Grammar
from tree_sitter import Node, Parser


def walk(node: Node) -> list[Node]:
    """Every node in the tree, parents before children."""
    found = [node]
    for child in node.children:
        found.extend(walk(child))
    return found


def tree_problems(root: Node, language: Language) -> list[str]:
    """Function and forbidden-construct problems anywhere in the tree."""
    problems = []
    for node in walk(root):
        if node.type in language.functions:
            problems.extend(function_problems(node, language))
        banned = forbidden_rust(node) if language is rules.RUST else None
        if banned:
            problems.append(f"{node.start_point.row + 1}: {banned} hides a failure")
    return problems


def file_problems(path: Path) -> list[str]:
    """Every shape problem in one file, or why it could not be checked."""
    language = rules.BY_SUFFIX.get(path.suffix)
    if language is None:
        return [f"1: no shape rules for `{path.suffix}` files"]
    try:
        content = path.read_bytes()
    except OSError as error:
        return [f"1: unreadable: {error}"]
    tree = Parser(Grammar(language.grammar)).parse(content)
    if tree.root_node.has_error:
        return ["1: does not parse"]
    problems = tree_problems(tree.root_node, language)
    lines = code_lines(content.decode(errors="replace"), language)
    if lines > rules.MAX_FILE_CODE_LINES:
        problems.append(
            f"1: file has {lines} code lines (max {rules.MAX_FILE_CODE_LINES})"
        )
    return problems


def main(paths: list[str]) -> int:
    """Reports every problem in every file; 1 when any was found."""
    failed = False
    for name in paths:
        for problem in file_problems(Path(name)):
            print(f"{name}:{problem}")
            failed = True
    return int(failed)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
