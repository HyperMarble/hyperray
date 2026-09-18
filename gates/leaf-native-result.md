# Gates: native search result validation

Scope: diagnostic result validation and automatic counterexample replay, not formal proof acceptance.

- [x] G1: Report and request decisions reject incomplete and inconsistent evidence.
  CHECK: go test -count=1 -v ./execution/nativecheck && echo NATIVE_RESULT_DECISIONS_OK
  EXPECT: NATIVE_RESULT_DECISIONS_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution/nativecheck	0.563s | NATIVE_RESULT_DECISIONS_OK

- [x] G2: All native and Cargo preparation cases use the public result API successfully.
  CHECK: go test -count=1 -tags preparation_integration -run 'Test(CargoPreparation|Preparation|NativeResult)' -v ./execution && echo NATIVE_RESULT_MATRIX_OK
  EXPECT: NATIVE_RESULT_MATRIX_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution	28.930s | NATIVE_RESULT_MATRIX_OK

- [x] G3: Go regression, static analysis, and race tests pass.
  CHECK: go test ./... && go vet -tags preparation_integration ./... && go test -race ./execution/nativecheck ./execution && echo NATIVE_RESULT_QUALITY_OK
  EXPECT: NATIVE_RESULT_QUALITY_OK
  EVIDENCE: ok  	github.com/HyperMarble/hyperray/execution	(cached) | NATIVE_RESULT_QUALITY_OK

- [x] G4: Public construction, source limits, and exact outcome boundaries pass review.
  EVIDENCE: Fourteen source files measured at most 72 lines per file and 35 lines per function. Public request construction passed in execution/nativecheck/request_test.go. All 21 generated checkers used the public Run API in /tmp/hyperray-native-result.Em47Ah/matrix.log. Different and missing replay executables and an incorrect boundary failed without publishing a reproduced counterexample. docs/native-result.md excludes proof acceptance, authenticated output, binary identity, and full semantic coverage.
