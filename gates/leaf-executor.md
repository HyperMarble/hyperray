# Gates: real counterexample confirmation

Scope: confirm formal witnesses in the exact frozen execution environment.

- [ ] G1: Reference, false-positive, and false-negative witnesses execute in a
  fresh isolated workspace with the declared toolchain, command, and signal.
  CHECK: go test ./tests -run 'TestCounterexampleExecutor(Reference|FalsePositive|FalseNegative|Baseline)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G2: Verdict files and exit codes are fresh and authoritative; stale or
  forged signals are rejected.
  CHECK: go test ./tests -run 'TestCounterexampleExecutorVerdict' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G3: Original bytes are restored after success, failure, timeout,
  interruption, and multi-file confirmation; path/symlink escapes are blocked.
  CHECK: go test ./tests -run 'TestCounterexampleExecutor(Restore|Containment|Isolated)' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending
- [ ] G4: Model/execution disagreement blocks verification and records enough
  evidence to diagnose the translator rather than hiding the mismatch.
  CHECK: go test ./tests -run 'TestCounterexampleExecutor.*Mismatch' -count=1
  EXPECT: /ok.*github.com\/HyperMarble\/ray\/tests/
  EVIDENCE: pending

