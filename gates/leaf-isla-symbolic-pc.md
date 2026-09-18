# Gates: symbolic instruction addresses

Scope: Preserve instruction-address evidence without program-specific rules.

- [x] G1: The real failing trace and its constraints have retained evidence.
  EVIDENCE: /tmp/hyperray-sequential-replay.441HmB/semantic.trace retains the generic-call output. Lines 2935 and 2953 show the final instruction and later symbolic PC read. Line 3027 fetches the return-boundary bytes without another instruction event. capture.log reproduces the parser error with the actual tool output.

- [ ] G2: A reusable connection preserves all feasible addresses or returns an error.
  EVIDENCE: Partial. The parser requires a concrete PC only when it consumes an instruction. A symbolic PC clears stale address state. Malformed symbolic names remain errors. Isla applies its existing zero-announcement exit before the instruction counter. The memory-overlap and sequential-read paths share a SAT-plus-uniqueness query. Both sequential program stages also use Sail's existing address partition with separate footprint configuration. The two-target function table passed all three queries. General address completeness remains open.

- [x] G3: The unchanged generic-call, dynamic-call, and recursion fixtures pass both queries.
  EVIDENCE: All three passed both queries in the full five-case repeat. Generic calls passed in 60.11 seconds with 23 observed instructions and counterexample 20. Dynamic dispatch passed in 66.76 seconds with 24 observed instructions and counterexample 12. Recursion passed in 161.17 seconds with 23 observed instructions and counterexample 6. /tmp/hyperray-unique-read.uazx9M/patterns.log retains the results. The separate two-visit recursion request returned an explicit error without a verdict in 66.12 seconds.

- [ ] G4: Multiple targets, conflicting evidence, and unreachable traces cannot produce false coverage.
  EVIDENCE: Partial. Six real trace fixtures pass: terminal symbolic PC, stale-address rejection, sibling isolation, used-symbol rejection, malformed-symbol rejection, and later concrete replacement. New solver tests preserve both alternatives, reject inconsistent constraints, and retain an address outside the concrete region. Default memory behavior and symbolic event replay passed. General multiple-target and unreachable-trace resolution remains open.
  The compiler-built function table passed the stronger output-set query and
  both outcome queries in 124.88 seconds. Both target functions had entry events.
  The shared arguments activate Sail's existing memory-address partition only
  for sequential program execution. Isolated instruction analysis retains its
  original configuration. The full repeat is in
  /tmp/hyperray-multiple-targets.IMhJIb/full-integration.log.

- [ ] G5: The complete integration replay and code-style checks pass.
  EVIDENCE: Partial. All fourteen listed real-tool tests passed across patterns.log, regressions.log, and integration-rest.log. A name-by-name comparison found no missing, failed, or skipped test. Package race tests passed in 10.957 seconds and go vet with isla_integration passed. The library passed 99 unit tests and three documentation tests. The complete upstream workspace tests passed. Rustfmt passed for the changed small Rust files, which contain at most 70 lines. Clippy reported no diagnostic in the unique-value or sequential-memory modules. Other upstream diagnostics remain, so the complete style gate remains open. Logs are in /tmp/hyperray-unique-read.uazx9M/.
  The later single command passed all fifteen listed real-tool tests in
  886.794 seconds. A fresh name comparison found no missing, failed, or skipped
  test. All Go packages passed go test -count=1 ./... . Integration-tagged
  go vet and the changed Go formatter checks passed. The clean reconstructed
  native workspace passed 140 tests. The reproducible-source leaf passed
  all five gates. Upstream warnings still prevent a warning-free claim.
