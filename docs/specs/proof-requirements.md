# Proof requirements

Frozen. The complete list of requirements spec.md content needs before hyperray's
finite checks eliminate false positives with full force. The schema's shape
already holds every one of these; they are content rules and validations.

Hyperray works on a bounded scope only, and only when it is finite. Every
requirement below preserves that: finite inputs D, finite outcomes O, so a
solution's behaviour is one finite table F : D -> O, and there are finitely
many tables. The proofs enumerate them. The bridges below are what make the
enumeration speak about real programs.

## A. The bound (held by the schema today)

- R1. Every parameter declares its complete finite value list (`Universe:`).
  A number range becomes labels; a free string becomes the finitely many
  cases the contract distinguishes.
- R2. Every operation's outcome vocabulary is closed: named outcomes plus
  `other outcome` as the complement.
- R3. Every value combination has exactly one row, `reachable` or `excluded`
  with a reason. Checked by spec-lint.

## B. The input bridge (table -> real program)

- R4. Every reachable row carries one real runnable input: an actual file,
  program, or argument list. A witness restating the row's labels carries
  nothing.
- R5. The task ships a classifier: an executable that maps a real input to
  its row. Total and deterministic over the scope.
- R6. `reachable` is earned: hyperray runs the witness through the classifier and
  confirms the row. A witness that does not land there blocks the task.
- R7. The task declares its finite scope of real inputs. Every input any
  test uses classifies into a row; an input outside the scope blocks the
  task.

## C. The output bridge (real result -> outcome label)

- R8. Every outcome label ships an observer: an executable predicate over
  the real output. `hoisted` is a checkable fact about the produced IR, not
  a word.
- R9. Observers are total and mutually exclusive per operation: every run
  observes exactly one outcome; `other outcome` is the automatic remainder.
- R10. Effects are observed the same way, never assumed.

## D. The meaning bridge (row -> contract) — the evidence rule

- R11. Every row anchors into the frozen instruction (contract) or frozen
  reference (mechanism). See evidence-rule.md.
- R12. Mechanism the contract does not owe stays ungraded; contract
  behaviour the reference lacks is a reference bug to report, never a row to
  soften.

## E. The test bridge (tests -> the same finite world)

- R13. Every test translates into: the inputs it runs, classified into rows
  by R5, and a verdict function over outcomes observed by R8. A test whose
  verdict depends on anything outside D and O blocks the task.
- R14. `Enforced by` is filled only from that translation. A name match is
  not evidence.
- R15. The four checks close by finite enumeration:
  - for every (row, forbidden outcome) pair, some translated test rejects
    it; a pair nothing rejects is the false-positive witness, named exactly;
  - every behaviour table satisfying the spec passes the translated tests,
    else the false-negative witness is named;
  - the reference, run on every row's witness, observes only required
    outcomes;
  - the translated tests accept the reference.

## Why this closes the gap

Any real solution inside the declared scope induces exactly one finite
behaviour table: the classifier assigns its inputs to rows, the observers
assign its results to outcomes. Translated tests see only that table. So
enumerating the finitely many tables covers every real solution in scope,
without sampling.

## The remaining trust surface

Two points stay human and are stated rather than hidden: reading the
contract (bounded by R11 anchors and independent review), and the
classifier/observer code itself, which is small, per-task, and reviewable.
