# Structured register source artifact

`patches/register-fields-v1.patch` applies directly to upstream commit
`7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
It includes the earlier memory, typed-state, and model-call changes.
It must not apply after those older patches.

The existing assertion grammar accepts nested scalar register fields.
The model declarations supply the field names and types.
One projection function supplies assertion values and counterexample values.
The production connection contains no Rust-program or trap-cause table.

All 71 exported files matched the patched clean checkout before the next change.
The final local build passed 154 native tests and all 23 real-tool tests.
The real-tool command completed in 558.796 seconds.
`measured-register-fields.json` records the executable and source identities.

This artifact does not establish full semantic coverage.
The total 100 MB target includes child tools and remains unmeasured.
