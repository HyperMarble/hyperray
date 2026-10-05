# Purpose: the repo's shape limits and, per language, which syntax counts.
# Never:   hold checking logic; a limit lives here once, read by the checker.
"""The repo's shape limits and the per-language syntax they apply to."""

from dataclasses import dataclass

import tree_sitter_go
import tree_sitter_python
import tree_sitter_rust

MAX_FILE_CODE_LINES = 75
MAX_FUNCTION_LINES = 40
MAX_NESTING = 2


@dataclass(frozen=True)
class Language:
    """What one language calls a function, a nesting block, and a comment."""

    grammar: object
    functions: frozenset[str]
    nesting: frozenset[str]
    comment_prefixes: tuple[str, ...]


RUST = Language(
    grammar=tree_sitter_rust.language(),
    functions=frozenset({"function_item"}),
    nesting=frozenset(
        {
            "if_expression",
            "for_expression",
            "while_expression",
            "loop_expression",
            "match_expression",
        }
    ),
    comment_prefixes=("//",),
)

PYTHON = Language(
    grammar=tree_sitter_python.language(),
    functions=frozenset({"function_definition"}),
    nesting=frozenset(
        {
            "if_statement",
            "for_statement",
            "while_statement",
            "with_statement",
            "try_statement",
            "match_statement",
        }
    ),
    comment_prefixes=("#",),
)

GO = Language(
    grammar=tree_sitter_go.language(),
    functions=frozenset({"function_declaration", "method_declaration"}),
    nesting=frozenset(
        {
            "if_statement",
            "for_statement",
            "expression_switch_statement",
            "type_switch_statement",
            "select_statement",
        }
    ),
    comment_prefixes=("//",),
)

BY_SUFFIX = {".rs": RUST, ".py": PYTHON, ".go": GO}

# Rust constructs that hide a failure; the code must return the error.
FORBIDDEN_RUST_METHODS = frozenset({"unwrap", "expect"})
FORBIDDEN_RUST_MACROS = frozenset({"panic"})
