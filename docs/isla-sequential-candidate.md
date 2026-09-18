# Sequential candidate constraints

The explicit sequential profile uses the existing Isla candidate pipeline.
The trace writer emits every retained SMT declaration, definition, and assertion.
These assertions include branch constraints and the byte-array memory callback.
Register initialization and reset processing remain in the shared setup path.

The sequential route adds the existing translated final assertion.
It does not add whole-access read-from constraints or CAT constraints.
Its model query requests final register symbols, not CAT relations.
The existing solver process and result reader remain authoritative.
The default axiomatic route remains unchanged.

Graph requests, extra SMT, read-from enumeration, and final-memory assertions
return explicit errors for this profile. These features require separate support.
The parent setup rejects unsupported initial-memory contracts and external effects.

Native evidence must include a satisfiable mixed-width execution and an
unsatisfiable changed result through the actual candidate assertion helper.
This integration does not prove full bounded-program coverage or CAT equivalence.

## Measured evidence

Four native tests passed with no ignored tests. The mixed-width query permits
`0x2211` from two byte writes. Its negation is unsatisfiable. The existing model
reader returns the concrete result from the emitted value query.
A constant-only query works without CAT names.

The three production binaries built successfully. Existing upstream warnings
remain. Public executable integration remains the parent task.
