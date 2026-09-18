# Isla semantic trace trees

The pinned commit is `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
The source is `isla-7f6882b/isla-lib/src/simplify.rs`.
`write_event_tree_with_opts` writes one trace prefix and then its child traces.
The `cases` expression contains a source-location string and one or more traces.
It is the final expression in its parent trace.
`write_events_in_context` writes each event as an expression.
Quoted strings, quoted register names, and source comments can contain parentheses.

Hyperray parses expressions rather than lines. It preserves the PC value from
the parent prefix separately for each child trace. A child cannot change the
PC value of its siblings. Each instruction consumes one concrete PC read.
An instruction within a nested event value is data, not an executed instruction.

Malformed framing, missing PC values, and symbolic PC values return errors.
The parser records every instruction event and the exact address-encoding set.
This record does not prove instruction meaning or full bounded-program coverage.

## Measured evidence

The original line parser returned zero instructions for both regression fixtures.
It returned no error. The fixtures contain sibling traces and multiline events.
The new parser records the exact address-encoding pairs in both fixtures.

The real symbolic-branch test passed with `rustc 1.98.0` and `LLD 22.1.8`.
Its static inventory contains six instructions. Its execution inventory contains
the same six instructions. The counterexample values are `17` and `64`.
The impossible output `65` has no satisfying execution in this query.
