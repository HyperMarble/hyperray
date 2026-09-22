# English view

The English view is one generated line of Python-level pseudocode.
It lets a person read the exact contract without reading solver syntax.

## Current direction

The direction is only:

```text
.hray contract -> English view
```

Hyper-Ray does not currently parse, store, or prove the English view.
The `.hray` contract remains the only source.

## Planned human authoring

A later human editor will accept only fixed pseudocode forms.
It will build the same contract tree and then write canonical `.hray` text.

```text
fixed human pseudocode -> contract tree -> .hray contract
```

This is not free English and does not use AI translation.
The current release accepts AI-written `.hray` contracts only.

## One-line rule

The complete view must occupy one line.
It starts with the universal input scope.
It then shows the function result, memory rules, operating-system rules, and requirements.

Example:

```text
for every arg1 in u64: advance(arg1) must return wrapping_add(arg1, 1)
```

## Fixed translation

Each grammar item has one fixed pseudocode form.
The renderer does not ask AI to write the view.

Examples:

| Formal item | Pseudocode item |
|---|---|
| `bvadd` | `wrapping_add` |
| `bvsub` | `wrapping_sub` |
| `bvult` | unsigned `<` |
| `bvslt` | signed `<` |
| `select` | `memory[address]` |
| `and` | `and` |
| `or` | `or` |
| `not` | `not` |

The view includes widths when a width changes the meaning.
It includes signedness when signedness changes the meaning.

## Completeness rule

Every accepted grammar item must have a view translation.
A missing translation is a build error.
An exhaustive renderer enforces this rule.

## Failure rule

If a valid contract cannot produce one view line, the renderer returns an error.
It never drops a contract part or substitutes a guessed phrase.
