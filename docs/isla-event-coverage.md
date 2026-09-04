# Isla Event Coverage

Status: TARGET DESIGN. The event inventory does not exist yet.

## Purpose

An Isla trace contains SMT formulas and generic machine-state events. The circuit stage must account for each event without an instruction-name table.

## Inventory

Hyperray reads every retained instruction trace. It records each top-level event in source order.

Each record contains these values:

- The instruction address.
- The trace index.
- The event index.
- The event kind.

The three indexes give each event a stable identifier. The event kind comes from the trace and is not a program-specific rule.

The reader accepts quoted text and quoted identifiers. It treats an Isla source-location suffix as annotation text, not an event.

Malformed syntax, text outside a trace, an empty event, or an empty trace causes an engine error.

## Translation coverage

The circuit translator gives each event one disposition:

- `translated` names one or more circuit regions.
- `proved_erasure` names a proof that the event has no state effect.

An unknown event kind has no default disposition. It causes an engine error.

The coverage validator requires these exact equalities:

```text
Isla event identifiers = translated event identifiers
circuit region identifiers = translated region identifiers
```

Thus, one fixed event grammar serves all instructions and all input programs. A new program changes the inventory, not Hyperray source.
