# Gates: exhaustive proof kernel

Scope: Return exact finite-state safety and termination verdicts with traces.

- [x] G1: A safe complete model returns PROVED.
  CHECK: go test ./proof -run TestProved
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.450s

- [x] G2: A violation returns DISPROVED with requirement, state, path, and cause.
  CHECK: go test ./proof -run TestDisprovedTrace
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.173s

- [x] G3: An incomplete certificate returns an error and no verdict.
  CHECK: go test ./proof -run TestIncompleteCoverageIsError
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.257s

- [x] G4: A reachable nonterminal cycle disproves termination with a cycle trace.
  CHECK: go test ./proof -run TestNonterminationCycle
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.162s

- [x] G5: Formatting and static checks pass.
  CHECK: gofmt -d proof && go vet ./proof
  EXPECT: /^$/
  EVIDENCE: `gofmt -d proof` and `go vet ./proof` both returned exit 0 with no output on 2026-09-05.

- [x] G6: Query roots change reachability but never change coverage counts.
  CHECK: go test ./proof -run 'TestQuerySpecificRoots|TestNonQueryRootStillRequiresCoverage'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.146s

- [x] G7: A bad transition returns an exact safety witness with state values.
  CHECK: go test ./proof -run TestDisprovedTransition
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.163s

- [x] G8: A terminal self-loop does not create a false termination failure.
  CHECK: go test ./proof -run TestTerminalStutterProves
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/proof	0.229s
