# Model-call safety source artifact

`patches/forbidden-model-calls-v1.patch` contains the cumulative local changes
from upstream commit `7f6882b3f468fe34df38ae3ffc0aede5baa0e7d6`.
This patch includes the earlier typed-state and sequential-memory connections.
It applies directly to the pinned commit, not after the earlier patches.

The new connection adds `forbidden_model_calls` to the generated program.
It reuses existing function-call events and the existing candidate solver.
The final safety query retains the original path constraints.
The public SDK requires `--forbidden-model-calls` from both whole-program tools.
`docs/isla-forbidden-calls.md` defines the contract and error conditions.

The patch applied to a clean checkout in
`/tmp/hyperray-forbidden-calls.sqR4AZ/clean-source/`.
All 60 exported files matched the working source byte for byte.
The local build used the working source and a separate target directory.
It did not overwrite earlier measured executables.

The patch is a source artifact, not a translation-equivalence proof.
It does not establish full instruction coverage, an operating-system model,
thread support, or the 100 MB target. Existing upstream warnings remain.
