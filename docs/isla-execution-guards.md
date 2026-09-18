# Shared instruction-visit guard

The trace stage and solver stage must use the same instruction-visit limit.
This limit controls proof resources. It does not define permitted program behavior.
The finite-state contract in `docs/proof-machine.md` permits cycles.
An exhausted visit limit returns an engine error, never a logic verdict.

The existing native `TaskState::with_pc_limit` supplies this guard.
The solver already selects `PCLimitMode::Error` through its public arguments.
The trace tool must accept `--pc-limit` and use the same native mechanism.
It must not expose a discard mode through this connection.
Missing limits retain standalone upstream behavior. Zero or malformed limits return errors.

The SDK passes its existing request limit to both tools.
The semantic evidence records that limit alongside the solver evidence.
Final result acceptance requires both recorded limits to equal the request limit.
An older trace tool must reject the new argument rather than ignore the guard.

The regression uses the existing compiler-built recursive program.
A low limit must stop the trace stage before a solver proposal exists.
A sufficient limit must retain the existing proof and changed-requirement results.
Separate unit tests exercise argument values and evidence mismatches.

This change does not implement fixed-point reachability or accepting-cycle proofs.
Those requirements remain open. It prevents inconsistent resource controls between stages.
