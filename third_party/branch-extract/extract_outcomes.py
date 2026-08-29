#!/usr/bin/env python3
"""Extracts every distinct terminal outcome -- every return, and every
raise/throw/panic -- from a real source file in any of hyperray's target
languages, for the sufficiency check (internal/sufficiency): does
spec.md's Required-behavior text account for every real outcome the
source can produce?

DESIGN: all language knowledge lives in _QUERIES below, declaratively,
in tree-sitter's own query language. There is deliberately no Python-side
traversal, no node-type lists, and no string munging. Everything after a
query match is language-agnostic: capture the matched node's verbatim
source text, and stop.

Three earlier versions were replaced to get here, each for a real reason:

  1. A hand-rolled walk over Python's builtin `ast` module. Only ever
     worked for Python -- useless for hyperray's Rust/C++/Go targets.
  2. A tree-sitter version that still hand-walked children looking for
     node types ("string", "type_identifier", ...) and munged qualified
     names with .split("::"). Those were guessed lists that would need a
     new special case per language and per value shape. Deleted after
     confirming, against the real sktime source, that matching spec.md's
     quoted phrases against verbatim source text alone reproduces the
     identical result -- so all of it was redundant.
  3. Semgrep, whose universal AST_generic promises one pattern for every
     language. Measured instead of assumed: it needs `return $X;` for C++
     but `return $X` elsewhere, and `throw $X;` vs `raise $X` -- so it
     does not remove per-language patterns, it relocates them, while
     adding a heavy, slow dependency. Rejected on evidence.

An outcome's value is its REAL SOURCE TEXT, verbatim, whatever its shape.
There is no per-shape interpretation: recognizing what a given source
text means is a judgment call for whoever reads it, and inventing a
heuristic per value shape is exactly the brute-forcing this avoids.

Prints one JSON object per line (JSONL), sorted by source line:
  {"kind": "raise"|"return", "line": N, "source_text": "<verbatim>"}
"""
import json
import sys

from tree_sitter import Language, Parser, Query

try:  # 0.25+ runs queries through a cursor; older bindings do not have it
    from tree_sitter import QueryCursor
except ImportError:  # pragma: no cover - depends on installed binding version
    QueryCursor = None

# The complete language-specific surface of this tool. Each query names
# node types from that language's own real, maintained grammar, verified
# by parsing real code rather than guessed. Adding a language means adding
# an entry here and nothing else.
_QUERIES = {
    "python": {
        "module": "tree_sitter_python",
        # (_) requires a child expression, so a bare `raise` re-raise and a
        # bare `return` are excluded by the grammar itself -- neither
        # carries text spec.md could ever match, so reporting them would
        # be a permanent, unfixable gap.
        "raise": "(raise_statement (_)) @node",
        "return": "(return_statement (_)) @node",
        # A generator's yield is a value the caller observes, so spec.md
        # has to account for it the same way it accounts for a return.
        # The grammar names both `yield x` and `yield from xs` `yield`.
        "yield": "(yield) @node",
    },
    "rust": {
        "module": "tree_sitter_rust",
        # Rust has no throw: the terminating macros are its raise. The
        # grammar only says "this is a macro_invocation" -- which macros
        # terminate is real language semantics no grammar encodes, so it
        # is declared here as a closed set, in the query itself.
        # println! and friends correctly do not match.
        # The `?` operator is a real early error-exit -- `risky()?` returns
        # Err immediately on failure. Found by testing real Rust rather
        # than assumed: without this, a Result-returning function's most
        # common error path is invisible.
        "raise": '[(macro_invocation macro: (identifier) @_m'
                 '  (#any-of? @_m "panic" "unreachable" "todo" "unimplemented"))'
                 ' (try_expression)] @node',
        # Rust's implicit trailing-expression return is NOT matched: it is
        # syntactically an ordinary expression, indistinguishable from a
        # non-returning one without type analysis the grammar does not do.
        "return": "(return_expression (_)) @node",
        "yield": "(yield_expression) @node",
        # A macro body is parsed as an opaque token_tree, so code inside
        # one is invisible to the queries above. tree-sitter-rust's OWN
        # shipped queries/injections.scm declares the remedy --
        #     ((macro_invocation (token_tree) @injection.content)
        #      (#set! injection.language "rust"))
        # -- i.e. re-parse a macro body as Rust. This matters acutely for
        # hyperray: Verus code is entirely wrapped in `verus! { ... }`, so
        # without injection the Rust verifier's own source yields zero
        # outcomes. Found by stress-testing real Verus sources, and fixed
        # using the grammar's declaration rather than a verus-specific
        # special case.
        # Both forms the grammar declares, not just the common one.
        # The capture must sit on the token_tree, not the macro_invocation:
        # capturing the whole macro would re-parse the identical text and
        # find nothing new.
        "inject": "[(macro_invocation (token_tree) @content)"
                  " (macro_rule (token_tree) @content)]",
    },
    "cpp": {
        "module": "tree_sitter_cpp",
        "raise": "(throw_statement (_)) @node",
        # co_return is a coroutine's return -- a real function exit that
        # the plain return_statement query does not match at all.
        "return": "[(return_statement (_)) (co_return_statement)] @node",
        "yield": "(co_yield_statement) @node",
        # No "inject" key on purpose. A function-like `#define` body can
        # contain real `return`s, and those are invisible here -- but
        # tree-sitter-cpp's own injections.scm deliberately declares only
        # raw-string injection, not preproc bodies, because a macro body
        # is frequently an unparseable fragment rather than standalone
        # code. Overriding that judgement would be guesswork. It is also
        # moot for hyperray's pipeline: ESBMC preprocesses C++ before
        # verifying, so macros are already expanded in what actually gets
        # checked. Found by stress-testing real system headers.
    },
    "go": {
        "module": "tree_sitter_go",
        # Go has no throw: a returned error is an ordinary return, so it
        # surfaces through the return query with its full multi-value
        # source text, e.g. `return 0, errors.New("...")`.
        #
        # Unlike the other three, Go's query has no (_): a naked `return`
        # in Go returns the function's NAMED result values, so it is a
        # real outcome carrying real values -- not the "returns nothing"
        # case that (_) excludes elsewhere. That is a genuine semantic
        # difference between the languages, so it is declared here rather
        # than normalized away.
        "return": "(return_statement) @node",
    },
}


def _captured_nodes(query, tree, capture="node"):
    """tree-sitter's capture API moved across binding versions: 0.25+ runs
    captures through a QueryCursor and returns a dict of capture-name ->
    nodes; older bindings called Query.captures directly and returned a
    list of (node, name) pairs. Support both rather than pinning hyperray to
    one binding version."""
    if QueryCursor is not None:
        captures = QueryCursor(query).captures(tree.root_node)
    else:
        captures = query.captures(tree.root_node)
    if isinstance(captures, dict):
        return captures.get(capture, [])
    return [node for node, name in captures if name == capture]


_MAX_INJECTION_DEPTH = 5


def _scan(source, lang, spec, parser, line_offset, depth, found):
    """Collect outcomes from one parse of `source`, then recurse into any
    injected regions the grammar declares (see the "inject" key). Line
    numbers are shifted by line_offset so nested results still point at
    the real file."""
    tree = parser.parse(source)

    for kind in ("raise", "return", "yield"):
        if kind not in spec:
            continue
        for node in _captured_nodes(Query(lang, spec[kind]), tree):
            found.append({
                "kind": kind,
                "line": node.start_point[0] + 1 + line_offset,
                "source_text": source[node.start_byte:node.end_byte].decode("utf-8", errors="replace"),
            })

    if "inject" not in spec or depth >= _MAX_INJECTION_DEPTH:
        return
    for node in _captured_nodes(Query(lang, spec["inject"]), tree, capture="content"):
        inner = source[node.start_byte:node.end_byte]
        # A macro body that is not real code simply yields no outcomes;
        # re-parsing it is harmless, so no guess about "is this code" is
        # needed.
        _scan(inner, lang, spec, parser, line_offset + node.start_point[0], depth + 1, found)


def extract(path, language_name):
    spec = _QUERIES[language_name]
    grammar = __import__(spec["module"])
    lang = Language(grammar.language())
    parser = Parser(lang)

    with open(path, "rb") as f:
        source = f.read()

    found = []
    _scan(source, lang, spec, parser, 0, 0, found)

    # Injection re-parses overlapping text, so the same outcome can be
    # collected more than once; keep one record per (line, text).
    seen = set()
    unique = []
    for record in sorted(found, key=lambda r: r["line"]):
        key = (record["line"], record["source_text"])
        if key in seen:
            continue
        seen.add(key)
        unique.append(record)
    return unique


def main():
    if len(sys.argv) != 3:
        print(f"usage: extract_outcomes.py <source-file> <{'|'.join(_QUERIES)}>", file=sys.stderr)
        sys.exit(2)
    path, language_name = sys.argv[1], sys.argv[2]
    if language_name not in _QUERIES:
        print(f"unsupported language {language_name!r}; want one of {sorted(_QUERIES)}", file=sys.stderr)
        sys.exit(2)
    for record in extract(path, language_name):
        print(json.dumps(record))


if __name__ == "__main__":
    main()
