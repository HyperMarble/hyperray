# spec.md — Compose probabilistic forecasts across FhPlex horizons

**In scope:** `RowwiseDistribution` (`sktime.base._proba`) — construction,
validation, per-cell statistics, `quantile`, `.loc`/`.iloc`/`.at`,
sampling (§1–4). `FhPlexForecaster`'s probabilistic surface
(`sktime.forecasting.compose`) — `_predict_proba`, `predict_var`,
`predict_quantiles`, `predict_interval`, capability-tag gating, `update`
forwarding (§5–9).

**Out of scope:** `FhPlexForecaster`'s pre-existing point-forecast
machinery (`_get_preds` and its callers `predict`/non-probabilistic
`predict_var`/etc.). Absolute-horizon behavior. Post-fit `set_params`
replacement. NaN/missing-value handling, anywhere. Sampling
reproducibility / `random_state`.

Each table below is preceded by its own `Parameters:` line declaring
every column's domain — the exact, disjoint set of atomic values that
column can hold. A table cell holds exactly one of: a single declared
value; a comma-separated list of declared values (a compound row,
covering each listed value); `any`, meaning every declared value of
that column applies equally; or `—`, meaning the column does not apply
to that row at all. No other free-text phrasing is used in a cell —
every value a row needs must be spelled out in the `Parameters:` line
first.

---

## 1. Construction

Parameters: `n_components` (0 / 1 / 2+), `component_kind` (labelled
array distribution / scalar / non-distribution object), `column_labels`
(identical / differ — value and order only, name-agnostic; only
meaningful when `n_components` is 2+ and `component_kind` is labelled
array distribution).

| n_components | component_kind | column_labels | Required behavior |
|---|---|---|---|
| 0 | — | — | raise `ValueError` containing "at least one component" |
| 1, 2+ | scalar | — | raise `ValueError` containing "labelled rows and columns" |
| 1, 2+ | non-distribution object | — | raise `TypeError` containing "probability distributions" |
| 1 | labelled array distribution | — | construct successfully; combined row index is that one component's index |
| 2+ | labelled array distribution | differ | raise `ValueError` containing "identical columns" |
| 2+ | labelled array distribution | identical | construct successfully; combined row index is the concatenation of each component's index, in component order |

Parameters: `input_container` (list / tuple).

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
`explicit_index_arg` (omitted / matching / mismatched — matching means
value and order match, name may differ).

| row_labels_after_concat | explicit_index_arg | Required behavior |
|---|---|---|
| duplicated | any | raise `ValueError` containing "row labels must be unique" |
| unique | mismatched | raise `ValueError` containing "Supplied index must match" |
| unique | matching | construct successfully; the **supplied** index object is retained, including its name |
| unique | omitted | construct successfully; the concatenated index is used as-is |

Parameters: `explicit_columns_arg` (omitted / matching / mismatched —
same definition as `explicit_index_arg`).

| explicit_columns_arg | Required behavior |
|---|---|
| mismatched | raise `ValueError` containing "Supplied columns must match" |
| matching | construct successfully; the **supplied** columns object is retained, including its name |
| omitted | construct successfully; the components' shared columns object is used as-is |

---

## 3. Per-cell statistics and quantiles

Parameters: `stat` (mean / var / pdf / log_pdf / cdf / ppf),
`frame_arg_row_col_order` (matches / reordered).

| stat | frame_arg_row_col_order | Required behavior |
|---|---|---|
| mean, var, pdf, log_pdf, cdf, ppf | matches | delegate to the owning component per row |
| mean, var, pdf, log_pdf, cdf, ppf | reordered | align the DataFrame argument by row and column **labels** before delegating to the owning component per row |

Parameters: `alpha_arg` (scalar / sequence).

| alpha_arg | Required behavior |
|---|---|
| scalar | accepted |
| sequence | accepted; requested order is preserved in the output column order |

| `quantile` property (any valid call) | Required behavior |
|---|---|
| column shape | variable-major `(column, alpha)` MultiIndex columns |
| cell values | each cell equals `ppf` for that row's owning component, at that variable and probability |

---

## 4. Row-ownership-preserving selection and sampling

Parameters: `selector` (`.loc` / `.iloc`), `row_selection` (reordered /
repeated / narrowed-only).

| selector | row_selection | Required behavior |
|---|---|---|
| .loc | reordered | preserve row ownership |
| .loc | repeated | preserve row ownership per repeated occurrence |
| .loc | narrowed-only | preserve row ownership |
| .iloc | reordered | preserve row ownership |
| .iloc | narrowed-only | preserve row ownership |
| .iloc | repeated | out of scope — not specified, must not be asserted either way |

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

Parameters: `wrapped_forecaster_capability` (capable / not capable),
`horizon_kind` (relative / absolute).

| wrapped_forecaster_capability | horizon_kind | Required behavior |
|---|---|---|
| capable | relative | return exactly one owned row per (series-or-panel-instance, horizon element); preserve variable order, panel-instance order (as first observed, not sorted), index names, and per-instance horizon sorting regardless of input order |
| capable | absolute | out of scope — no behavior asserted here |
| not capable | any | see §8 for the capability-gate behavior |

Parameters: `forecaster_realism` (mock / real `NaiveForecaster`).

| forecaster_realism | Required behavior |
|---|---|
| mock | point-forecast routing (predict/predict_var/etc.) is unaffected by probabilistic use, before and after |
| real NaiveForecaster | follows the identical routing path as the mock — no special-casing for real forecasters |

---

## 6. Forwarding to routed forecasters, and `update`

Parameters: `call` (predict_proba / update), `update_params` (True /
False — only meaningful for `update`; True is also the default when
omitted).

| call | update_params | Required behavior |
|---|---|---|
| predict_proba | — | forward a value-equal `X` (not necessarily the same object) and the exact `marginal` flag to every routed forecaster |
| update | True | forward equivalent `y` and `X` to every **existing** routed instance, reusing those same instance objects (no re-instantiation) |
| update | False | in addition to the above: forward the exact `update_params=False` flag; fitted parameters stay unchanged; the cutoff still advances |

| Property (any fitted instance) | Required behavior |
|---|---|
| `forecasters_` | routed instances are accessible via this attribute, keyed by horizon |

---

## 7. Contiguous mode and `fh_params`

Parameters: `fh_contiguous` (True / False), `horizon_value` (>1 / ≤1 —
only meaningful when `fh_contiguous` is True).

| fh_contiguous | horizon_value | Required behavior |
|---|---|---|
| True | >1 | that horizon's routed forecaster evaluates on the contiguous span 1 through it, but only its own assigned row is kept in the final output; panel output and mixed-horizon routing are still preserved |
| True | ≤1 | no contiguous expansion needed (span 1 through ≤1 is trivial) — same output-row behavior as above |
| False | any | no contiguous-span evaluation; each horizon routes independently |

Parameters: `fh_params_form` (mapping / list / callable / string
expression / default).

| fh_params_form | Required behavior |
|---|---|
| mapping | supported |
| list | supported |
| callable | supported |
| string expression | supported |
| default | supported, uses the documented default |

Parameters: `set_params_target` (forecaster / fh_params /
fh_contiguous), `set_params_timing` (pre-fit / post-fit).

| set_params_target | set_params_timing | Required behavior |
|---|---|---|
| forecaster, fh_params, fh_contiguous | pre-fit | routing is affected by the replacement |
| forecaster, fh_params, fh_contiguous | post-fit | out of scope — no behavior asserted |

| Sampling, after any of the above | Required behavior |
|---|---|
| — | each routed forecaster's joint instance-variable sampling block is preserved |

---

## 8. Capability-tag propagation and gating

Parameters: `wrapped_overall_capability` (True / False),
`wrapped_insample_capability` (True / False / absent — only
independently meaningful when `wrapped_overall_capability` is True;
absent means the wrapped forecaster never set the tag).

| wrapped_overall_capability | wrapped_insample_capability | Required behavior |
|---|---|---|
| False | True, False, absent | overall capability tag is False; in-sample capability tag is **forced** False regardless of the wrapped forecaster's own in-sample tag value or its absence |
| True | False | overall capability tag is True; in-sample capability tag is False; out-of-sample-only requests still work |
| True | True, absent | both tags effectively True; all requests work |

If the wrapped forecaster is replaced post-construction, capability
tags re-track the newly wrapped forecaster's actual capabilities using
the same rule above.

Parameters: `overall_capability` (True / False), `request_kind`
(in-sample / out-of-sample / mixed), `insample_supported` (True / False
— only meaningful when `overall_capability` is True and `request_kind`
is in-sample or mixed; this is the same value as
`wrapped_insample_capability` above, effectively True).

| overall_capability | request_kind | insample_supported | Required behavior |
|---|---|---|---|
| False | any | — | `predict_proba`, `predict_var`, `predict_quantiles`, `predict_interval` raise `NotImplementedError` containing "does not have the capability" |
| True | in-sample, mixed | False | raise `NotImplementedError` containing "in-sample prediction" |
| True | in-sample, mixed | True | proceed normally |
| True | out-of-sample | — | proceed normally |

---

## 9. Composed variance / quantiles / intervals

Parameters: `output` (predict_var / predict_quantiles /
predict_interval), `series_shape` (univariate / multivariate /
mixed-horizon).

| output | series_shape | Required behavior |
|---|---|---|
| predict_var | any | equals the composed variance |
| predict_quantiles | any | equals the composed quantiles; columns are unnamed `(variable, alpha)` MultiIndex |
| predict_interval | any | columns are unnamed `(variable, coverage, lower-or-upper)` MultiIndex; central interval bounds equal the composed quantiles |
