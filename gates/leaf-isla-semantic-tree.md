# Gates: Isla semantic trace trees

Scope: Preserve instruction addresses through the pinned event-tree grammar.
This leaf does not establish full bounded-program coverage.

- [x] G1: The grammar and state-copy rule have pinned source evidence.
  EVIDENCE: Commit 7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6, isla-lib/src/simplify.rs:1901-1935 writes prefixes before terminal cases and clones context per child. Lines 1626 and 1855 define trace and instr framing. Both original regression fixtures returned zero events with no error.
- [x] G2: Layout, sibling, nested-value, and malformed-tree regressions pass.
  CHECK: go test -count=1 ./machine/isla -run 'TestSemanticTree'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/machine/isla	0.543s
- [x] G3: The real compiler symbolic-branch proof passes.
  EVIDENCE: Final TestRealRustSymbolicBranchInput passed in 14.44s. Static instructions 6, observed 6, not observed 0. Counterexamples 0:x10=#x0000000000000011; and 0:x10=#x0000000000000040;. The impossible output 65 returned PROVED.
- [x] G4: Package race tests, static analysis, and source limits pass.
  CHECK: go test -count=1 -race ./machine/isla && go vet ./machine/isla
  EXPECT: ok
  EVIDENCE: Final package race tests passed in 7.509s. go vet passed with and without isla_integration. gofmt reported no files. The maximum file length is 70 lines, including existing semantic_threads.go. The maximum function length is 31 lines. No fourth-level indentation, panic, unwrap, or expect call occurs in the owned Go files.
