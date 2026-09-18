# Final register fields

Final assertions can name a scalar field in a structured model register.
For example, `0:mcause.bits = 7` refers to the model's declared `bits` field.
Nested paths use the same dot notation.
The existing assertion parser resolves field names from the model symbol table.
The shared setup validates each path against the declared register type before execution.

One projection function selects the final stored field for both the assertion
translator and the counterexample decoder. It does not compute instruction behavior.
The existing solver receives the original constraints and the selected scalar.
Symbolic fields remain symbolic until the solver supplies their values.
The model-value request includes symbols inside structured registers.

Unknown registers, unknown fields, fields on scalar values, missing final values,
and non-scalar terminal fields return errors. No error becomes a false assertion.
The supported terminal values are bitvectors and Booleans.
The existing register and memory assertion forms retain their syntax.
The Go SDK passes the assertion without a second field parser.
Older tools reject the new syntax instead of ignoring it.

The human trace format supports the new assertion location.
The current Coq export returns an explicit error for this location because its
existing output contract does not contain a register-field constructor.
It must not substitute the whole register or emit a successful empty result.

This connection does not establish full instruction semantics or exit classification.
The test requirements include nested fields, symbolic witnesses, type errors,
changed requirements, and a compiler-built architectural fault cause.

## Measured integration

The final build passed 154 native tests and all 23 real-tool tests.
The real-tool command completed in 558.796 seconds.
The public SDK returned `PROVED` for the declared cause `7`.
The changed cause requirement returned `DISPROVED` with `0:mcause.bits=#x0000000000000007;`.
The combined safety query retained both that cause and `called:trap_handler=true;`.

The expected cause comes from the pinned model, not the solver observation.
`riscv_model_rv64d.ir` maps `E_SAMO_Access_Fault` to `0x07` at lines 11772–11773.
The compiler-built fixture makes the store at the declared invalid stack address.
The test does not add a production instruction rule.

The initial native regression rejected a Boolean as unsupported.
Boolean fields are supported by this contract, so the unsupported-value case now uses `Val::Unit`.
A separate solver test requires the supported Boolean result.

`tools/isla/measured-register-fields.json` identifies the final source and binaries.
The preserved evidence is in `/tmp/hyperray-register-fields.JnFBoC/`.
