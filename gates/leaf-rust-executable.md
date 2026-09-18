# Gates: Direct Rust compiler to executable proof

Scope: Exercise the existing proof API on compiler-built Rust executables.
Separate static coverage from query execution without claiming unreachable code.

- [x] G1: Preserve all static instructions and report unobserved instructions.
  CHECK: go test -count=1 ./machine/isla -run 'TestProgramSemantics|TestExecutableVerifier|TestExecutableJoin'
  EXPECT: /^ok\s+github.com\/HyperMarble\/hyperray\/machine\/isla/m
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/machine/isla	0.514s
- [x] G2: Compile two Rust sources with rustc and use one unchanged proof API.
  EVIDENCE: TestRealRustExecutablePrograms passed on 2026-09-05 in 21.94 seconds.
  rustc 1.98.0 (88d9e12ae 2026-08-18) compiled branch.rs and arithmetic.rs.
  LLD 22.1.8 linked both ELF images. BuildProgram and VerifyProgram accepted them.
  Before the fix, zero_branch failed with a missing instruction at 0x80100002.
- [x] G3: Obtain correct-property results and false-property counterexamples.
  EVIDENCE: All three cases returned PROVED for the correct property and DISPROVED
  with a counterexample state for the changed property. zero_branch retained six
  static instructions with three observed. division_branch retained six with four
  observed. arithmetic retained four with four observed.
- [x] G4: Pass regression, race, formatting, and static analysis checks.
  EVIDENCE: go test -count=1 -race ./... returned exit 0 on 2026-09-05.
  go vet ./... and go vet -tags isla_integration ./machine/isla returned exit 0.
  gofmt -l machine/isla and git diff --check returned no output.
  rustfmt --check accepted both new Rust source fixtures.

The tests use concrete input registers and a declared return boundary.
They do not establish full Rust, operating-system, or source-language coverage.
