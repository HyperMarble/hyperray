# Gates: machine translation and circuit coverage

Scope: Prove that every accepted Sail machine case has exactly the same bit-precise
behavior in the Hyperray circuit, without loss or additions.

- [ ] G1: A pinned Sail build generates the complete accepted decode and semantic-case catalogs.
  EVIDENCE: pending

- [x] G2: The pinned Sail compiler lowers the complete typed model to JIB without an omitted or unsupported constructor.
  CHECK: ./tools/sail-jib/run.sh /Volumes/Hak_SSD/sail-riscv /tmp/sail-riscv-audit.g2Odln/opam-root/default/bin/sail
  EXPECT: /{"complete":true,"jib_definitions":1735,"jib_origins":167953,"instruction_kinds":14}/
  EVIDENCE: The pinned upstream C backend lowered the complete RV64 base-I profile. The grammar census visited 1,735 JIB definitions and 167,953 constructor origins. It emitted artifact SHA-256 `a0df439fb9be58269ab215e17101c991d44afb65c6000cc0f8bddb5e6fd33818`.

- [ ] G3: The pinned Sail SystemVerilog lowerer maps every JIB constructor, and no circuit region exists without one JIB origin.
  EVIDENCE: pending

- [ ] G4: A Lean theorem proves that JIB evaluation equals SystemVerilog circuit evaluation for every JIB constructor.
  EVIDENCE: pending

- [ ] G5: The certificate pins the Sail compiler and SystemVerilog backend until translation theorems replace this trust.
  EVIDENCE: pending

- [ ] G6: Exact catalog equality covers each decoded instruction, Sail case, semantic IR case, and circuit region in both directions.
  EVIDENCE: pending

- [ ] G7: Initial state, next state, traps, memory effects, labels, observations, terminal state, and external-contract choices have proved translations.
  EVIDENCE: pending

- [ ] G8: Small-model differential tests agree for every state, and a changed equation makes its Lean theorem fail.
  EVIDENCE: pending

- [ ] G9: A stale digest, missing case, extra case, unsupported Sail construct, or failed theorem returns an engine error and no coverage certificate.
  EVIDENCE: pending

- [ ] G10: The official BTOR2 parser accepts each emitted circuit.
  EVIDENCE: pending

- [ ] G10A: A changed executable or finite bound uses the same translator binary and generates new exact catalogs without a code change.
  EVIDENCE: pending

- [x] G11: Public APIs are reachable, buildable, callable, and observable from external tests.
  CHECK: go test -count=1 ./circuit -run 'TestPublicMiter|TestVariablePublicValues|TestBalanceMutationDifference|TestProposalRejectsInvalidPublicInputs|TestDifferenceEngineIdentity'
  EXPECT: ok
  EVIDENCE: ok github.com/HyperMarble/hyperray/circuit

- [x] G12: All circuit tests, formatters, static analysis, and source limits pass.
  CHECK: go test -count=1 -race -cover ./circuit && test -z "$(gofmt -d circuit)" && go vet ./circuit && echo PASS
  EXPECT: /coverage: 100.0%[\s\S]*PASS/
  EVIDENCE: coverage: 100.0% of statements | PASS

## First equivalence slice

- [x] S1: Public constructors build validated finite bit-vector step relations and deterministic miters.
  CHECK: go test -count=1 ./circuit -run 'TestPublicMiterIsDeterministic|TestRelationValidation'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/circuit	0.150s

- [x] S2: Z3 finds the exact least input for the changed 8-bit balance comparison.
  CHECK: go test -count=1 ./circuit -run 'TestBalanceMutationDifference|TestProposalIsDeterministic'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/circuit	0.336s

- [x] S3: Equal 8-bit balance relations return only an explicit unvalidated-UNSAT result.
  CHECK: go test -count=1 ./circuit -run TestEqualBalanceRelationIsUnvalidatedUnsat
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/circuit	0.155s

- [x] S4: Exhaustive fixture measurement covers all 8-bit balance and cost pairs.
  CHECK: go test -count=1 ./circuit -run TestBalanceFixturesExhaustive
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/circuit	0.208s

- [x] S5: Tool identity, process errors, and unsupported solver output are explicit.
  CHECK: go test -count=1 ./circuit -run 'TestZ3Identity|TestToolErrors|TestUnsupportedSolverOutput'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/circuit	0.391s

- [x] S6: Race analysis reports 100 percent statement coverage for the circuit package.
  CHECK: go test -count=1 -race -cover ./circuit
  EXPECT: /coverage: 100.0%/
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/circuit	2.473s	coverage: 100.0% of statements

- [x] S7: Formatting and static analysis accept the circuit package.
  CHECK: test -z "$(gofmt -d circuit)" && go vet ./circuit && echo PASS
  EXPECT: PASS
  EVIDENCE: PASS

- [x] S8: All handwritten circuit files obey the source-shape and hidden-failure limits.
  CHECK: test -z "$(find circuit fixtures/circuit -type f -exec awk 'FNR > 75 { print FILENAME; exit }' {} +)" && awk '/^func / { start = FNR; signature = $0 } /^}$/ && start { if (FNR - start + 1 > 40) { print FILENAME ":" start ": " signature; bad = 1 } start = 0 } END { exit bad }' circuit/*.go && awk '/^\t\t\t+(if|for|switch|select)[ (]/ { print FILENAME ":" FNR ": " $0; bad = 1 } END { exit bad }' circuit/*.go && ! rg -n '\b(interface|panic|recover)\b|os\.Exit|log\.Fatal|TODO|FIXME|func[[:space:]]+[^ (]+\[|_[[:space:]]*=' circuit fixtures/circuit && echo PASS
  EXPECT: PASS
  EVIDENCE: PASS

## Official integer translation slice

- [x] S9: The source gate accepts only the pinned type, instruction, and Lean support revisions.
  CHECK: ./proof/sail_addi/shell/run.sh /Volumes/Hak_SSD/sail-riscv /tmp/sail-riscv-audit.g2Odln/opam-root/default/bin/sail /tmp/lean-sail-v4 /tmp/lean429-local/lean-4.29.0-darwin_aarch64/bin/lake
  EXPECT: /Integer proofs passed. All one-bit mutations failed as required./
  EVIDENCE: exit 0

- [x] S10: The official catalogs have six I-type, two U-type, three shift-immediate, and ten R-type operations.
  CHECK: the S9 command prints all four catalog axiom reports
  EXPECT: all catalog reports depend only on recorded Lean axioms
  EVIDENCE: all four reports contain `[propext]`

- [x] S11: Every generated encoding in the five proved families returns the same instruction through the official decoder.
  CHECK: the S9 command prints all five decoder axiom reports
  EXPECT: all decoder reports depend only on recorded Lean axioms
  EVIDENCE: I-type, R-type, shift-immediate, and ADDIW use `[propext, Classical.choice, Quot.sound]`; U-type uses `[propext, Quot.sound]`

- [x] S12: All five generated execution functions equal their finite state actions for all 22 operations and all finite inputs.
  CHECK: the S9 command prints all five execution bridge axiom reports
  EXPECT: all bridge reports depend only on recorded Lean axioms
  EVIDENCE: all five reports contain `[propext, Classical.choice, Quot.sound]`

- [x] S13: All five instruction values equal their independent circuits for all 22 operations and all finite inputs.
  CHECK: the S9 command prints all five circuit equivalence axiom reports
  EXPECT: all circuit reports depend only on recorded Lean axioms
  EVIDENCE: I-type, U-type, and R-type use `[propext, Quot.sound]`; shift-immediate and ADDIW use `[propext]`

- [x] S14: One-bit mutations for all five proved families do not pass and give concrete counterexamples.
  CHECK: the S9 command operates all five mutation files and requires nonzero results
  EXPECT: all mutation logs contain /found a counterexample/
  EVIDENCE: ADDI, U-type, R-type, shift-immediate, and ADDIW mutations returned finite assignments

- [x] S15: The proof slice obeys the shell, source-shape, and hidden-failure limits.
  CHECK: bash -n proof/sail_addi/shell/run.sh proof/sail_addi/shell/source.sh proof/sail_addi/shell/measure.sh && test -z "$(find proof/sail_addi -type f -exec awk 'FNR > 75 { print FILENAME; exit }' {} +)" && ! rg -n 'sorry|admit|panic|unwrap|expect|let _' proof/sail_addi && echo PASS
  EXPECT: PASS
  EVIDENCE: PASS

- [x] S16: The pinned typed Sail tree has exact declaration, decoder, and execution ownership for every RV64 base-I instruction family.
  CHECK: ./tools/sail-catalog/run.sh /Volumes/Hak_SSD/sail-riscv /tmp/sail-riscv-audit.g2Odln/opam-root/default/bin/sail
  EXPECT: /{"complete":true,"instruction_families":23}/
  EVIDENCE: exact set equality accepted 23 instruction families; catalog SHA-256 `763943c54a5edc7374508a5bdc72c63fe7408e5d42075e588e2b811d44e0b777`

- [x] S17: The full JIB traversal gives every constructor occurrence one stable origin identifier.
  CHECK: ./tools/sail-jib/run.sh /Volumes/Hak_SSD/sail-riscv /tmp/sail-riscv-audit.g2Odln/opam-root/default/bin/sail
  EXPECT: /{"complete":true,"jib_definitions":1735,"jib_origins":167953,"instruction_kinds":14}/
  EVIDENCE: The validator accepted 167,953 unique origins. Each used constructor kind has at least one origin.

- [x] S18: Exact JIB-to-circuit catalog validation rejects missing, extra, duplicate, and implicit mappings.
  CHECK: go test -count=1 -race -cover ./coverage/jibcatalog
  EXPECT: /coverage: 100.0%/
  EVIDENCE: coverage: 100.0% of statements

The family catalog proves ownership, not instruction meaning. The RV64 Lean
slice proves 22 operations in five families. Its family-local meaning coverage
is 5/23, or 21.7%. Gates G1 through G10 remain open until the same meaning
proof and exact catalogs cover the accepted machine profile.

Production code must not dispatch on names such as `ADDI`, `RTYPEW`, or
`SHIFTIWOP`. It can dispatch only on the fixed JIB grammar. Thus, one semantic
rule covers all source instructions that Sail lowers to that JIB constructor.

Production code also must not contain fixture names, program addresses,
program bytes, source function names, or selected bounds. These values are
inputs to the generated proof instance.
