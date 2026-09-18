# Gates: architecture documents

Scope: State the complete proof claim and the universal reference route.

- [x] G1: The design separates semantic coverage, reachability, and verdict.
  CHECK: rg -n "Semantic coverage|Reachability|Verdict" docs/design.md docs/proof-machine.md
  EXPECT: Semantic coverage
  EVIDENCE: `docs/design.md:56-103` defines all three values separately.

- [x] G2: Only complete coverage permits a logic verdict.
  CHECK: rg -n "PROVED|DISPROVED|engine error" docs/design.md docs/proof-machine.md docs/stage3.md
  EXPECT: engine error
  EVIDENCE: `docs/proof-machine.md:275-277` requires complete coverage and returns engine errors for incomplete certificates.

- [x] G3: The reference route includes source, compiler, machine, environment, and finite transition semantics.
  CHECK: rg -n "compiler|machine|environment|transition" docs/proof-machine.md
  EXPECT: transition
  EVIDENCE: `docs/proof-machine.md:80-88` states the route. Lines 99-188 define its compiler, machine, environment, and transition parts.

- [x] G4: Kani and other language tools are accelerators, not coverage authorities.
  CHECK: rg -n "accelerator|coverage authorit" docs/design.md docs/proof-machine.md docs/stage3.md
  EXPECT: accelerator
  EVIDENCE: `docs/proof-machine.md:358-385` limits language tools to checked accelerator results.

- [x] G5: The document states the mathematical reason that exhaustive finite-state verification is a proof.
  CHECK: rg -n "fixed point|induction|reachable state|accepting cycle" docs/proof-machine.md
  EXPECT: fixed point
  EVIDENCE: `docs/proof-machine.md:278-356` gives the fixed-point induction and accepting-cycle arguments.

- [x] G6: The documents make no claim that the current implementation has complete Rust or multi-language coverage.
  EVIDENCE: `docs/stage3.md:3` marks Stage 3 replaced and incomplete. Lines 245-266 list current Rust gaps.

- [x] G7: Stage 4 includes the graph and circuit routes, and it does not make Kani the prover.
  CHECK: test -z "$(rg -n 'The prover is Kani' docs/stage4.md)" && rg -n "explicit finite graph" docs/stage4.md && rg -n "symbolic sequential circuit" docs/stage4.md
  EXPECT: symbolic sequential circuit
  EVIDENCE: `docs/stage4.md:60` names the explicit finite graph reference. Line 62 names the symbolic sequential circuit route. The stale-authority search returned no match.

- [x] G8: The semantic coverage design defines all generated catalogs, exact equalities, and the Stage 4 stop rule.
  CHECK: rg -n "compiler-operation catalog" docs/semantic-coverage.md && rg -n "executable-instruction catalog" docs/semantic-coverage.md && rg -n "ISA/EEI semantic-case catalog" docs/semantic-coverage.md && rg -n "circuit-region catalog" docs/semantic-coverage.md && rg -n "root catalog" docs/semantic-coverage.md && rg -n "provenance" docs/semantic-coverage.md && rg -n "exact set equalities" docs/semantic-coverage.md && rg -n "Stage 4 cannot start" docs/semantic-coverage.md
  EXPECT: Stage 4 cannot start
  EVIDENCE: `docs/semantic-coverage.md:43-102` defines all six catalogs. Lines 141-169 give exact equalities. Line 11 gives the Stage 4 stop rule.
