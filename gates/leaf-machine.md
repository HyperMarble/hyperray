# Gates: finite proof machine

Scope: Define total machine and environment semantics for one exact profile.

- [x] G1: The loader accepts only the named executable profile and records every loaded byte.
  CHECK: go test -count=1 ./machine -run 'TestLoad|TestInstructions|TestRejects'
  EXPECT: ok
  EVIDENCE: External tests load the real static LP64D fixture. They compare every file-backed and zero-filled PT_LOAD byte with an independent debug/elf parse. Adversarial fixtures and named header mutations return public rejection codes. Instruction records only frame bytes. They do not state instruction meaning.

- [ ] G2: The generated ISA catalog equals the implemented machine-semantic catalog.
  EVIDENCE: No pinned Sail catalog or machine-semantic catalog exists in this slice.

- [ ] G3: Every executable address decodes to a supported semantic case or a declared trap.
  EVIDENCE: The loader records 16-bit and 32-bit encodings. It does not decode an opcode or declare a trap.

- [ ] G4: Every allowed environment action has every declared finite result and state effect.
  EVIDENCE: The finite Linux-user environment does not exist in this slice.

- [ ] G5: Memory, thread, scheduler, and environment capacities have explicit boundary behavior.
  EVIDENCE: The loader has an explicit loaded-byte limit. Machine memory, threads, scheduling, and environment state do not exist in this slice.

- [ ] G6: Every proof root has an exact machine-state predicate and a complete domain certificate.
  EVIDENCE: No machine-state predicate or domain certificate exists in this slice.

- [ ] G7: Unknown instructions, actions, and missing semantic cases return engine errors.
  EVIDENCE: Unknown ELF forms and unsupported instruction lengths return exact loader errors. Instruction meaning, environment actions, and semantic-case errors do not exist yet.

- [ ] G8: Each machine artifact records its inputs, tool identity, and SHA-256 digest.
  EVIDENCE: The public image records the full ELF SHA-256, profile, and loaded-byte limit. It does not record compiler or linker identity, so this gate remains open.

- [x] G9: Public APIs are reachable, buildable, callable, and observable from external tests.
  CHECK: go test -count=1 ./machine -run 'TestLoadRecordsArtifactIdentity|TestLoadRecordsEveryLoadableByte|TestInstructionsPartitionExecutableBytes'
  EXPECT: ok
  EVIDENCE: Package machine_test calls machine.Load and observes Image, LoadedByte, ExecutableRegion, Instruction, Rejection, and RejectionCode values.

- [x] G10: All machine tests, formatters, static analysis, and source limits pass.
  CHECK: go test -count=1 -race -cover ./machine && test -z "$(gofmt -d machine)" && go vet ./machine && echo PASS
  EXPECT: /coverage: 100.0%[\s\S]*PASS/
  EVIDENCE: The measured result was 100.0 percent statement coverage and PASS.

  CHECK: test -z "$(find machine fixtures/machine -type f -exec awk 'FNR > 75 { print FILENAME; exit }' {} +)" && awk '/^func / { start = FNR; signature = $0 } /^}$/ && start { if (FNR - start + 1 > 40) { print FILENAME ":" start ": " signature; bad = 1 } start = 0 } END { exit bad }' machine/*.go && awk '/^\t\t\t+(if|for|switch|select)[ (]/ { print FILENAME ":" FNR ": " $0; bad = 1 } END { exit bad }' machine/*.go && ! rg -n '\b(interface|panic|recover)\b|os\.Exit|log\.Fatal|TODO|FIXME|func[[:space:]]+[^ (]+\[|_[[:space:]]*=' machine fixtures/machine && echo PASS
  EXPECT: PASS
  EVIDENCE: The file, function, nesting, and forbidden-pattern scan printed PASS.
