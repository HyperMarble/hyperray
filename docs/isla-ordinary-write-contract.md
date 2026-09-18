# Ordinary write contract

This contract applies to the pinned RISC-V sequential profile.
It does not define a generic failure signal for every memory system.

The local Sail source is commit
`5745ea9e5369ab4fc51de6f8b773dd8ebc323357`.
`lib/concurrency_interface/read_write_v1.sail:215` assigns the returned Boolean
to exclusive-write or compare-and-swap success.
The source wraps this Boolean in `Ok(Some(b))` at line 254.
An ordinary store does not use this Boolean as a permission-fault result.

The compiled authority is
`isla-snapshots/riscv_model_rv64d.ir`, snapshot commit
`d1f2098b94b5b101e4f467de429733e00c0c7975`.
Its `files` record maps source index 14 to `read_write_v1.sail`.
The ordinary primitive call is at IR line 7930.
`zsail_mem_write` returns `Ok(Some(b))` at IR lines 7986–7987.
`zwrite_ram` accepts `Ok(_)`, ignores the Boolean payload, and returns true
at IR lines 8024–8032.

Faults use a different path.
`zchecked_mem_write` starts at IR line 38936.
It reports physical-access errors before it calls `zphys_mem_write`.
It also distinguishes memory-mapped input/output and physical RAM.
Thus, a false primitive Boolean cannot represent an ordinary RAM fault here.

Isla's `AxEvent.write_data` reads the store bytes and data, not its Boolean.
The candidate encoder includes ordinary write events without that Boolean guard.
The sequential byte array must represent the same ordinary store.

The callback therefore updates every ordinary store that passes validation.
It does not constrain the unused Boolean to true.
It rejects exclusive accesses and tags before the update.
A memory error preserves the previous bytes and returns an execution error.
Invalid-address and fault paths remain in the ISA and memory-error paths.

The earlier conditional update was incorrect for this profile.
Its failed-ordinary-write test validated an invented contract.
The replacement test permits either Boolean value and requires the stored bytes
in both cases. Separate error tests require unchanged prior bytes.
