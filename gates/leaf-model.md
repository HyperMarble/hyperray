# Gates: finite transition model

Scope: Provide a public, constructible, deterministic, validated finite graph model.

- [x] G1: External-package tests construct and validate the public API.
  CHECK: go test ./model
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/model	(cached)

- [x] G2: Validation results do not depend on state or transition order.
  CHECK: go test ./model -run 'TestStateValidationOrder|TestTransitionValidationOrder|TestValidationReferences'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/model	(cached)

- [x] G3: Public graph identifiers obey the on-disk v1 identifier pattern.
  CHECK: go test ./model -run 'TestIdentifier'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/model	(cached)

- [x] G4: Every canonical model fixture decodes, validates, and re-encodes exactly.
  CHECK: go test ./model -run TestCanonicalModelFixtures
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/model	(cached)

- [x] G5: Exact values and the nil-or-empty rule have deterministic JSON.
  CHECK: go test ./model -run 'TestStateValuesRoundTrip|TestNilAndEmptyValues'
  EXPECT: ok
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/model	(cached)

- [x] G6: Race analysis and statement coverage pass together.
  CHECK: go test -race -cover ./model
  EXPECT: /coverage: 100.0%/
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/model	(cached)	coverage: 100.0% of statements

- [x] G7: Formatting and static analysis pass.
  CHECK: test -z "$(gofmt -d model)" && go vet ./model && echo PASS
  EXPECT: PASS
  EVIDENCE: PASS

- [x] G8: File, function, nesting, and forbidden-pattern limits pass.
  CHECK: test -z "$(find model -name '*.go' -exec awk 'FNR > 75 { print FILENAME; exit }' {} +)" && awk '/^func / { start = FNR; signature = $0 } /^}$/ && start { if (FNR - start + 1 > 40) { print FILENAME ":" start ": " signature; bad = 1 } start = 0 } END { exit bad }' model/*.go && awk '/^\t\t\t+(if|for|switch|select)[ (]/ { print FILENAME ":" FNR ": " $0; bad = 1 } END { exit bad }' model/*.go && ! rg -n '\b(interface|panic|recover)\b|os\.Exit|log\.Fatal|TODO|FIXME|func[[:space:]]+[^ (]+\[|_[[:space:]]*=' model && echo PASS
  EXPECT: PASS
  EVIDENCE: PASS
