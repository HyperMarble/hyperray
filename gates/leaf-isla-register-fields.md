# Gates: final structured register fields

Scope: Reuse declared model structures in assertions and concrete counterexamples.

- [x] G1: The existing grammar preserves nested paths and setup rejects invalid declared paths.
  EVIDENCE: register_field_tests.rs covers nested paths, exact displayed paths, unknown fields, wrong members, scalar traversal, aggregate endpoints, absent threads, and non-register roots. All six field tests passed within the 154-test native workspace result in /tmp/hyperray-register-fields.JnFBoC/native-formatted.log.

- [x] G2: Assertions and counterexamples select the same concrete or symbolic field. Missing values return errors.
  EVIDENCE: register_field_value_tests.rs and register_field_solver_tests.rs passed. The real solver proves the constrained symbolic value 7, rejects the changed value 8, decodes the same witness, and proves a Boolean field. Runtime missing values, invalid traversal, and aggregate endpoints return errors. Both production consumers call register_fields::value.

- [x] G3: The public SDK proves the compiler-built trap cause and rejects a changed requirement with that cause as evidence.
  EVIDENCE: TestRealRustStructuredTrapCause, TestRealRustStructuredCauseWithForbiddenTrap, and TestRealRustRejectsMissingRegisterField passed in real-formatted.log. The selected cause is 0:mcause.bits=#x0000000000000007;. The combined query retains called:trap_handler=true;. The missing-field query returns an error without a verdict.

- [x] G4: Native and Go regressions, real-tool tests, changed-source style, and source reconstruction pass.
  EVIDENCE: 154 native tests passed. All Go packages, race tests, and vet passed. All 23 real tests passed in one 558.796-second command. Changed-file rustfmt and Go formatting passed. New native modules had no selected Clippy diagnostics. Existing upstream warnings and formatting differences outside the changed sections remain. The cumulative patch applied to a clean checkout, and all 71 files matched. measured-register-fields.json records the final hashes.
