# Gates: Symbolic input through the existing machine model

Scope: Measure a compiler-built branch over the model's symbolic register domain.
This does not add a source-pattern bound or claim universal source coverage.

- [x] G1: Record the model source of the symbolic input and its width.
  EVIDENCE: Pinned Isla init.rs:136 creates UVal::Uninit. register.rs:121
  creates a symbolic value on the first read. The pinned model declares
  `register zx10 : %bv64` at line 14635. TestRealRustSymbolicBranchInput
  requires a symbolic x10 read in the retained semantic trace.
- [x] G2: One query preserves both compiler-generated branch paths.
  CHECK: go test -count=1 -tags isla_integration ./machine/isla -run '^TestRealRustSymbolicBranchInput$' -v
  EXPECT: static instructions: 6; observed: 6; not observed: 0
  EVIDENCE: PASS | ok  	github.com/HyperMarble/hyperray/machine/isla	16.682s
  The symbolic case retained all six static instructions across both paths.
- [x] G3: The solver rejects an impossible result and finds both branch outcomes.
  EVIDENCE: The symbolic test passed in 12.67 seconds with exact-value assertions.
  Query x10=65 returned PROVED for its negation. Queries x10=17 and x10=64
  returned DISPROVED with 0:x10=#x0000000000000011; and
  0:x10=#x0000000000000040;. These are final states, not replayable inputs.
- [x] G4: Regression and static analysis checks pass after the change.
  CHECK: go test -count=1 -race ./...
  EXPECT: /^ok\s+github.com\/HyperMarble\/hyperray\/machine\/isla/m
  EVIDENCE: ?   	github.com/HyperMarble/hyperray/tools/sail-catalog/go	[no test files] | ok  	github.com/HyperMarble/hyperray/tools/sail-jib/go	1.412s
  go vet -tags isla_integration ./machine/isla returned exit 0.
  gofmt -l machine/isla and git diff --check returned no output.
  All four native final_value tests passed. The release build passed.
  The upstream build retains pre-existing compiler warnings.
  rustfmt --check accepted both added upstream Rust files.
  git apply --reverse --check accepted the recorded dependency patch.

The reproducible patch and build commands are in tools/isla/.
measured-release.json records the measured binary, model, and patch digests.
The dependency patch requests final values and labels candidate outcomes.
It does not add Rust-specific rules or alter solver constraints.

This leaf establishes the stated bounded queries through the trusted Isla route.
It does not establish full Rust coverage or an independent proof of Isla.
