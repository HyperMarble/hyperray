# spec.md — Compose probabilistic forecasts across FhPlex horizons

Derived from `instruction.md` (the live Shipd Task Prompt for challenge
`kh78jk7jm20efgwwmb8kah430s8b33an`). Each section below is one
independently-checkable clause of that prompt, expressed as a condition
table: every row is one combination of the clause's discrete parameters,
mapped to the one required behavior. `ray spec-lint` checks each table
for **completeness** (every combination has exactly one row) and
**disjointness** (no two rows conflict) before any other layer runs.

Two objects are in scope: `RowwiseDistribution` (`sktime.base._proba`)
and `FhPlexForecaster` (`sktime.forecasting.compose`). Sections 1–4 cover
`RowwiseDistribution`; sections 5–9 cover `FhPlexForecaster`.

---

## 0. Decisions the prompt leaves implicit

This is the actual work of writing this spec: the prompt reads as
complete prose, but it does not settle these five points. Each is a
real judgment call, made here explicitly rather than left for whoever
writes tests or the solution to silently assume — which is exactly how
three of the four real bugs in this task's original solution got in
(see the regression notes in sections 2, 5, and 8: each is a place
where an implicit assumption diverged from what the prompt actually
required, undetected because nothing forced the question to be asked
before code existed).

1. **Is "identical column labels required" (§1, component-to-component)
   name-sensitive?** The prompt says components need "identical column
   labels required in order" to combine, but separately says an
   *explicit* `columns` argument must match "including names" on
   retention (§2) — implying name is retained but was not necessarily
   part of *that* match test either. Decision: **name-agnostic** here
   too, for consistency with §2's resolved behavior and because nothing
   in the prompt singles out inter-component matching as stricter than
   the explicit-argument case. §1's table is written accordingly.

2. **What does "forward an equivalent `X`" mean (§6), given `update`
   in the same paragraph is specified as forwarding the *exact* `y` and
   `X`?** The prompt's own word choice shifts from "exact" to
   "equivalent" for `predict_proba`'s `X`. Decision: **"equivalent"
   means value-equal, not necessarily the identical object** — a
   per-instance/per-horizon slice of the caller's `X` that has the same
   values as what that routed forecaster should see is compliant, even
   if it isn't the literal same object passed to `FhPlexForecaster`.
   This is weaker than `update`'s "exact... reusing those objects,"
   deliberately, matching the prompt's own distinct wording.

3. **Capability tag state when in-sample capability is absent
   entirely** (not `True` or `False`, just unset) **on the wrapped
   forecaster.** Not addressed anywhere in the prompt. Decision: treated
   identically to `True` for the purposes of §8's forcing rule — an
   absent in-sample tag is not evidence of self-normalization, so
   overall-`False` must still force it `False`. Any wrapped forecaster
   that never sets the tag is exactly the "does not self-normalize"
   case the regression note in §8 already covers.
4. **NaN / missing values anywhere in the pipeline** — never mentioned
   by the prompt, for either `RowwiseDistribution` or
   `FhPlexForecaster`. Decision: **explicitly out of scope.** No row in
   any table below asserts NaN behavior; a test that exercises NaN
   input is testing something this spec does not cover, and per this
   project's own gap rule, that would itself be an unfair test, not a
   missing spec clause.
5. **Sampling reproducibility / `random_state`.** The prompt never
   mentions determinism. Decision: **out of scope** — §4's sampling
   rows describe structural properties of `sample()`'s output (which
   labels, which grouping, where sample ids appear), not value
   reproducibility across calls.

---

## 1. Construction

Parameters: `n_components` (0 / 1 / 2+), `component_kind` (labelled array
distribution / scalar / non-distribution object), `input_container`
(list / tuple), `column_labels_across_components` (identical, in order /
differ in order or value).

| n_components | component_kind | column_labels | Required behavior |
|---|---|---|---|
| 0 | — | — | raise `ValueError` containing "at least one component" |
| 1+ | scalar | — | raise `ValueError` containing "labelled rows and columns" |
| 1+ | non-distribution object | — | raise `TypeError` containing "probability distributions" |
| 2+ | labelled array distribution | differ in value or order (name-agnostic — see §0.1) | raise `ValueError` containing "identical columns" |
| 1+ | labelled array distribution | identical in value and order (name-agnostic — see §0.1) | construct successfully; combined row index is the concatenation of each component's index, in component order |

Additional, orthogonal to the table above:

| input_container | Required behavior |
|---|---|
| list | accepted, same result as tuple |
| tuple | accepted, same result as list |

| Property (any successfully constructed instance) | Required behavior |
|---|---|
| clonability | `clone()` reproduces an equivalent, independently usable instance |

---

## 2. Row/column label validation and retention

Parameters: `row_labels_after_concat` (unique / duplicated),
`explicit_index_arg` (omitted / supplied matching / supplied
mismatched), `explicit_columns_arg` (omitted / supplied matching /
supplied mismatched — value+order match but possibly different name).

| row_labels_after_concat | explicit_index_arg | Required behavior |
|---|---|---|
| duplicated | any | raise `ValueError` containing "row labels must be unique" |
| unique | mismatched (value or order) | raise `ValueError` containing "Supplied index must match" |
| unique | matching (value + order; name may differ) | construct successfully; the **supplied** index object is retained, including its name |
| unique | omitted | construct successfully; the concatenated index is used as-is |

| explicit_columns_arg | Required behavior |
|---|---|
| mismatched (value or order) | raise `ValueError` containing "Supplied columns must match" |
| matching (value + order; name may differ) | construct successfully; the **supplied** columns object is retained, including its name |
| omitted | construct successfully; the components' shared columns object is used as-is |

**Regression note (bug found and fixed this session):** "matching"
above is defined by **value and order only** — name is explicitly *not*
part of the match condition, and a differently-named-but-otherwise-equal
`columns` argument must still be accepted and its name retained. The
original solution's `_columns_metadata_match` used `Index.identical()`
(name-sensitive), which rejected this case — contradicting this table's
last two rows. Fixed by switching to `.equals()` (value+order only).

---

## 3. Per-cell statistics and quantiles

Parameters: `stat` (`mean` / `var` / `pdf` / `log_pdf` / `cdf` / `ppf` /
`quantile`), `frame_arg_row_col_order` (matches combined order /
reordered).

| stat | frame_arg_row_col_order | Required behavior |
|---|---|---|
| mean, var, pdf, log_pdf, cdf, ppf | matches | delegate to the owning component per row |
| mean, var, pdf, log_pdf, cdf, ppf | reordered | align the DataFrame argument by row and column **labels** before delegating to the owning component per row |

| quantile: `alpha` arg | Required behavior |
|---|---|
| scalar | accepted |
| sequence, any order | accepted; requested order is preserved in the output column order |

| quantile: property (any valid call) | Required behavior |
|---|---|
| column shape | variable-major `(column, alpha)` MultiIndex columns |
| cell values | each cell equals `ppf` for that row's owning component, at that variable and probability |

---

## 4. Row-ownership-preserving selection and sampling

Parameters: `selector` (`.loc` / `.iloc` / `.at`), `row_selection`
(reordered / repeated / narrowed-only, N/A for `.at`), `column_selection`
(narrowed / full).

| selector | row_selection | Required behavior |
|---|---|---|
| .loc | reordered | preserve row ownership (each returned row keeps its originating component's distribution behavior) |
| .loc | repeated | preserve row ownership per repeated occurrence |
| .iloc | reordered | preserve row ownership |
| .iloc | narrowed-only (no reorder/repeat) | preserve row ownership |

Note: `.loc` supports repeated-row selection; `.iloc` is specified only
for reordered/narrowed selection (repetition not specified for `.iloc`
— per completeness, this combination is out of this table's scope and
must not be asserted either way).

| .at[row, column] | Required behavior |
|---|---|
| any valid single row/column pair | returns the matching **scalar** distribution; its `mean`, `var`, `pdf`, `log_pdf`, `cdf`, `ppf`, `sample` are themselves scalars, not arrays/frames |

Parameters: `sample_call` (`sample()` / `sample(n_samples=k)`).

| sample_call | Required behavior |
|---|---|
| sample() | retains the currently-selected labels; preserves each component's joint row-variable sampling behavior; samples each component separately |
| sample(n_samples=k) | all of the above, plus: sample ids appear as the outer level of a MultiIndex on the result |

---

## 5. FhPlexForecaster — probabilistic row identity and ordering

Parameters: `wrapped_forecaster_capability` (capable of `pred_proba` /
not capable), `horizon_kind` (relative / absolute), `panel_kind`
(single series / panel with 2+ instances, non-alphabetical order),
`forecaster_realism` (mock / real `NaiveForecaster`).

| wrapped_forecaster_capability | horizon_kind | Required behavior |
|---|---|---|
| capable | relative | return exactly one owned row per (series-or-panel-instance, horizon element); preserve variable order, panel-instance order (as first observed, not sorted), index names, and per-instance horizon sorting regardless of input order |
| capable | absolute | out of this clause's scope (relative-only per this task's descoping — no behavior asserted here) |
| not capable | any | out of this clause's scope (see section 8 for the capability-gate behavior) |

| forecaster_realism | Required behavior |
|---|---|
| mock | point-forecast routing (predict/predict_var/etc.) is unaffected by probabilistic use, before and after |
| real NaiveForecaster | follows the identical routing path as the mock — no special-casing for real forecasters |

**Regression note (bug found and fixed this session):** the original
solution combined rows via `combined.index.argsort()` — a lexicographic
**value** sort, which silently reorders panel instances alphabetically
(e.g. observed order `("z", "a")` would incorrectly become `("a", "z")`
in the output), contradicting "preserving... panel-instance order"
above. Fixed via a `_panel_order()` helper: rank instances by first
observed appearance, then `np.lexsort` for time-within-instance
ordering only. This fix is scoped to `_predict_proba` only — `_get_preds`
(used by `predict`/`predict_var`/`predict_quantiles`/`predict_interval`)
is pre-existing, untouched code, out of this task's scope, and was
confirmed via `git show <base-commit>` to already exist before this
task's solution patch.

---

## 6. Forwarding to routed forecasters, and `update`

Parameters: `call` (`predict_proba` / `update`), `update_params` (N/A /
True / False).

| call | update_params | Required behavior |
|---|---|---|
| predict_proba | N/A | forward a value-equal `X` (see §0.2 — not necessarily the same object) and the exact `marginal` flag to every routed forecaster |
| update | (unspecified) | forward equivalent `y` and `X` to every **existing** routed instance, reusing those same instance objects (no re-instantiation) |
| update | False | in addition to the above: forward the exact `update_params=False` flag; fitted parameters stay unchanged; the cutoff still advances |

| Property (any fitted instance) | Required behavior |
|---|---|
| `forecasters_` | routed instances are accessible via this attribute, keyed by horizon |

---

## 7. Contiguous mode and `fh_params`

Parameters: `fh_contiguous` (True / False), `horizon_value` (>1 / ≤1,
only meaningful when True), `fh_params_form` (mapping / list / callable
/ string expression / default), `set_params_target` (`forecaster` /
`fh_params` / `fh_contiguous` / none), `set_params_timing` (pre-fit /
post-fit).

| fh_contiguous | horizon_value | Required behavior |
|---|---|---|
| True | >1 | that horizon's routed forecaster evaluates on the contiguous span 1 through it, but only its own assigned row is kept in the final output; panel output and mixed-horizon routing are still preserved |
| True | ≤1 | no contiguous expansion needed (span 1 through ≤1 is trivial) — same output-row behavior as above |
| False | any | no contiguous-span evaluation; each horizon routes independently |

| fh_params_form | Required behavior |
|---|---|
| mapping | supported |
| list | supported |
| callable | supported |
| string expression | supported |
| default (omitted) | supported, uses the documented default |

| set_params_target | set_params_timing | Required behavior |
|---|---|---|
| forecaster | pre-fit | routing is affected by the replacement |
| fh_params | pre-fit | routing is affected by the replacement |
| fh_contiguous | pre-fit | routing is affected by the replacement |
| any of the above | post-fit | out of this clause's scope — no behavior asserted (prompt specifies pre-fit only) |

| Sampling, after any of the above | Required behavior |
|---|---|
| — | each routed forecaster's joint instance-variable sampling block is preserved |

---

## 8. Capability-tag propagation and gating

Parameters: `wrapped_overall_capability` (True / False),
`wrapped_insample_capability` (True / False, only independently
meaningful when overall is True), `forecaster_replaced` (never /
replaced post-construction), `request_kind` (in-sample / out-of-sample /
mixed).

| wrapped_overall_capability | wrapped_insample_capability | Required behavior |
|---|---|---|
| False | (any) | overall capability tag is False; in-sample capability tag is **forced** False regardless of the wrapped forecaster's own in-sample tag value |
| True | False | overall capability tag is True; in-sample capability tag is False; out-of-sample-only requests still work |
| True | True | both tags True; all requests work |

| forecaster_replaced | Required behavior |
|---|---|
| replaced post-construction | capability tags re-track the newly wrapped forecaster's actual capabilities |

**Regression note (bug found and fixed this session):**
`__dynamic_tags__` cloned `capability:pred_int` and
`capability:pred_int:insample` straight from the wrapped forecaster
without normalization — so a wrapped forecaster with overall capability
False but in-sample capability left True (a realistic case; not every
forecaster self-normalizes) leaked `capability:pred_int:insample=True`,
contradicting row 1 above. Fixed: `if not
self.get_tag("capability:pred_int"): self.set_tags(**{"capability:pred_int:insample":
False})`.

| overall capability | request needs in-sample | Required behavior |
|---|---|---|
| False | (any) | `predict_proba`, `predict_var`, `predict_quantiles`, `predict_interval` raise `NotImplementedError` containing "does not have the capability" |
| True | mixed horizon needs in-sample and it's unsupported | raise `NotImplementedError` containing "in-sample prediction" |
| True | not needed, or supported | proceed normally |

---

## 9. Composed variance / quantiles / intervals

Parameters: `output` (`predict_var` / `predict_quantiles` /
`predict_interval`), `series_shape` (univariate / multivariate /
mixed-horizon).

| output | Required behavior (all series_shape values) |
|---|---|
| predict_var | equals the composed variance |
| predict_quantiles | equals the composed quantiles; columns are unnamed `(variable, alpha)` MultiIndex |
| predict_interval | columns are unnamed `(variable, coverage, lower-or-upper)` MultiIndex; central interval bounds equal the composed quantiles |

Applies identically across univariate, multivariate, and mixed-horizon
forecasts — no row in this table varies by `series_shape`, which is
itself the completeness check: any solution that special-cases one
`series_shape` for one `output` violates this table.
