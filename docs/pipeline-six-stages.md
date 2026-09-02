# Hyperray pipeline — the six stages

Status: ACCEPTED by user — 2026-09-02
Companion to `finalarchitecture.md` (FROZEN, 2026-08-27). That file says what
Hyperray must prove. This file says the order in which it is done and what
each step is, in plain words. Per-language tools live in
`docs/<lang>_adapter.md`.

Stages 5 and 6 ARE test generation. Stages 1–4 are what make the generated
tests provable instead of guessed.

| # | stage | one line |
|---|---|---|
| 1 | EXTRACT | Read the code by machine: every function, type, constant, branch. No guessing. |
| 2 | BOUND | For every loop, find the rule that stays true every turn (e.g. `len <= 65536`), so the proof never has to run the loop forever. |
| 3 | PROVE | Ask the solver: is there ANY input that breaks this code? Answer is "none exists" or one exact input that does. |
| 4 | ADEQUACY | Break the code on purpose, re-run the proof. If the proof still passes, the spec is too weak — a row is missing. |
| 5 | PLAN | From all the true facts, keep only the ones the prompt may state: declared in the code, never observed by running it. |
| 6 | EMIT | Write the test file and `instruction.md` from that one picked list, in one pass, so they can never disagree. |

Data flow:

```
solution.patch + base checkout
  -> [1] extraction manifest (lang-adpaters/PROTOCOL.md)
  -> [2] manifest + Invariants column
  -> [3] obligations discharged, or a counterexample
  -> [4] surviving mutants -> missing rows
  -> [5] plan.json
  -> [6] test.patch + instruction.md
```

Names:

- **Hyperray** — the whole six-stage framework.
- **ember** — stage 1 only (the extractor; emits the manifest).
- Stages 2–4 are external tools per language (see adapter docs).
- Stages 5–6 have no name yet and are not built.

Status on 2026-09-02: stages 1–4 measured on Rust (noodles-296); stages 5–6
not built in any language.
