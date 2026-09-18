# Gates: semantic coverage validation

Scope: Reconcile exact evidence for the explicit-graph reference path.

- [x] G1: A complete certificate mints an opaque token with exact counts.
  CHECK: go test -count=1 ./coverage -run 'TestCompleteCertificate|TestValidatedCoverageZeroValue'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.243s

- [x] G2: Arbitrary stale output, instruction, rule, and provenance references fail.
  CHECK: go test -count=1 ./coverage -run TestStaleMappingReference
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.197s

- [x] G3: Unused reverse-catalog items fail explicitly.
  CHECK: go test -count=1 ./coverage -run 'TestUnusedCatalogEntry|TestUnusedProvenanceEdge|TestUnusedArtifact|TestUnusedProofCatalogs'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.260s

- [x] G4: Root evidence must equal the independent entry-state catalog.
  CHECK: go test -count=1 ./coverage -run 'TestRootSubset|TestRootSuperset'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.187s

- [x] G5: Conflicting paths fail, but shared semantic identity is valid.
  CHECK: go test -count=1 ./coverage -run 'TestConflictingReverseMapping|TestSharedTransitionSemanticIdentity'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.299s

- [x] G6: Valid elimination passes, and stale elimination evidence fails.
  CHECK: go test -count=1 ./coverage -run 'TestValidElimination|TestInvalidElimination'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.462s

- [x] G7: An unreachable operation stays in semantic coverage.
  CHECK: go test -count=1 ./coverage -run TestUnreachableOperation
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.255s

- [x] G8: Canonical public JSON fixtures round-trip without changes.
  CHECK: go test -count=1 ./coverage -run TestJSONFixtureRoundTrip
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.249s

- [x] G9: Formatting, static analysis, source shape, and forbidden patterns pass.
  CHECK: test -z "$(gofmt -d coverage)" && go vet ./coverage && test -z "$(find coverage -name '*.go' -exec awk 'FNR > 75 { print FILENAME; exit }' {} +)" && awk '/^func / { start = FNR; signature = $0 } /^}$/ && start { if (FNR - start + 1 > 40) { print FILENAME ":" start ": " signature; bad = 1 } start = 0 } END { exit bad }' coverage/*.go && awk '/^\t\t\t+(if|for|switch|select)[ (]/ { print FILENAME ":" FNR ": " $0; bad = 1 } END { exit bad }' coverage/*.go && ! rg -n '\b(interface|panic|recover)\b|os\.Exit|log\.Fatal|TODO|FIXME|func[[:space:]]+[^ (]+\[|_[[:space:]]*=' coverage && echo PASS
  EXPECT: PASS
  EVIDENCE: PASS

- [x] G10: A root has exact states or an independent impossible-precondition proof.
  CHECK: go test -count=1 ./coverage -run 'TestValidImpossibleRootProof|TestEmptyRootWithoutProof'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.174s

- [x] G11: Digest-bound artifacts and schema identifiers reject malformed data.
  CHECK: go test -count=1 ./coverage -run 'TestBadArtifactDigest|TestArtifactContentDigestMismatch|TestStaleArtifact|TestMalformedIdentifier'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.167s

- [x] G12: Permuting input does not change acceptance or the first public error.
  CHECK: go test -count=1 ./coverage -run 'TestInputOrderInvariant|TestValidInputPermutation|TestValidatedRootErrorOrder|TestUnsupportedProofOrderInvariant'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.506s

- [x] G13: The opaque token binds the exact model and validated root states.
  CHECK: go test -count=1 ./coverage -run 'TestValidatedCoverageSnapshotIgnoresInputMutation|TestValidatedCoverageCanonicalModel|TestValidatedCoverageModelDeepCopy|TestValidatedRootsCanonicalSelection|TestValidatedRootSelectionFailures|TestZeroValidatedCoverageRejectsRoots'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/coverage	0.191s
