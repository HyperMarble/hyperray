# Imported Sail memory proofs

These proofs use upstream memory functions directly.
They do not define instruction rules or replace the memory implementation.
The theorem parameters represent addresses, values, and memory states.

Replay the proofs:

```sh
bash proof/sail_memory/shell/run.sh \
  /Volumes/Hak_SSD/hyperray-research/sail-riscv-lean-proof-audit \
  /tmp/lean429-local/lean-4.29.0-darwin_aarch64/bin/lake
```

The replay requires exact model and support revisions without local changes.
Lean guards the recorded axiom dependencies of each theorem.
The permitted dependencies are `propext`, `Classical.choice`, and `Quot.sound`.
An unexpected axiom or warning fails the replay.
The replay also requires rejection of a changed read-after-write claim.
It does not certify the Sail exporter, the Isla memory callback, or full coverage.
Those connections remain obligations in `docs/lean-proof-route.md`.
