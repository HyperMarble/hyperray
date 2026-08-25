# spec.md examples

Real tables, confirmed to pass `ray spec-lint`, across different kinds
of tasks — not variations on one domain. Use these as starting points
for the grammar patterns they show, not as templates to copy verbatim.

## A simple enum, no wildcards needed

From a cron-expression field parser. Every value gets its own row —
the plainest, most common shape.

```markdown
Parameters: `atom_kind` (wildcard / single-integer / inclusive-range / stepped-range / stepped-wildcard).

| atom_kind | Required behavior |
|---|---|
| wildcard | every value from field minimum to field maximum |
| single-integer | that single integer, if in range; else error |
| inclusive-range | inclusive range [a, b]; a > b is an error |
| stepped-range | inclusive range [a, b] stepped by s from a; includes b iff (b - a) is a multiple of s |
| stepped-wildcard | every s-th value starting at the field minimum |
```

## `any` — one row applies regardless of another column

From a semver range resolver's prerelease opt-in rule. When
`version_has_prerelease` is `no`, the second column doesn't matter —
`any` says so explicitly instead of writing two near-duplicate rows.

```markdown
Parameters: `version_has_prerelease` (yes / no), `group_has_matching_base_prerelease_comparator` (yes / no).

| version_has_prerelease | group_has_matching_base_prerelease_comparator | Required behavior |
|---|---|---|
| no | any | opt-in rule never blocks; normal comparator evaluation applies |
| yes | yes | AND-group may match, subject to normal comparator evaluation |
| yes | no | AND-group rejects the version |
```

## `—` — a column that doesn't apply to a specific row

From a UI dispatch board's selection-persistence rule. Whether the
selected call still matches the active filters only matters when the
triggering action *was* a filter change — for every other action kind,
that column is meaningless, not "any value of it."

```markdown
Parameters: `action_kind` (filter change / sort change / edit change / reload), `selected_call_still_matches_filters` (yes / no).

| action_kind | selected_call_still_matches_filters | Required behavior |
|---|---|---|
| filter change | yes | selection unchanged; inspector shows the selected call |
| filter change | no | selection unchanged even though its row is hidden; inspector still shows the selected call |
| sort change | — | selection and inspector target unchanged |
| edit change | — | selection stays on the edited call |
| reload | — | edits, selection, filters, and sorting are all restored |
```

## A `/`-separated compound row

From ts-pattern's `matchEach`. Four different terminal calls share
identical behavior on the "no pattern matched" branch — one compound
row instead of four near-duplicates. Note the separator is `/`, the
same one the `Parameters:` line uses — never a comma, since a value's
own name might need one (see the next example).

```markdown
Parameters: `terminal_call` (run / exhaustive / toFunction / toExhaustiveFunction / exhaustive-with-fallback / otherwise / toPartialFunction), `match_result` (some matched / none matched).

| terminal_call | match_result | Required behavior |
|---|---|---|
| run / exhaustive / toFunction / toExhaustiveFunction | none matched | throw NonExhaustiveError |
| exhaustive-with-fallback | none matched | call fallback, return single-element array of its result |
| otherwise | none matched | return single-element array of the otherwise handler's result |
| toPartialFunction | none matched | return undefined |
| run / exhaustive / toFunction / toExhaustiveFunction / exhaustive-with-fallback / otherwise / toPartialFunction | some matched | return array of all matching results |
```

## An explicit catch-all bucket (not `any`)

From a CSV parser's quote-escaping rule. `other` is a real, named
value in the declared domain — not the wildcard keyword — because the
row genuinely means "none of the specific cases," which is a different
thing from "every value applies here."

```markdown
Parameters: `char_after_closing_quote` (delimiter / LF / CR / another-quote / other).

| char_after_closing_quote | Required behavior |
|---|---|
| delimiter | field ends normally |
| LF | row ends |
| CR | row ends |
| another-quote | doubled-quote escape: literal quote, field continues |
| other | raise CsvFormatError |
```

## What doesn't belong in a table

A value's own name can contain a comma (`"wildcard, all values"` is a
single value, not two) — but never a `/`, since that's the reserved
separator. If a parameter doesn't reduce to a short, disjoint list of
named buckets — a raw numeric range like "any float," a raw string —
name a bucketed property of it instead (`positive` / `non-positive`,
`empty` / `non-empty`), the same judgment call as naming any other
parameter. `ray spec-lint` rejects a `Parameters:` entry with no
`/`-separated value list rather than guessing what you meant.
