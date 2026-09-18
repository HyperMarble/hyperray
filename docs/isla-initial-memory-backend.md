# Initial memory backend

`Memory.initialize_section(address, bytes, writable)` records each initial byte.
Writable sections use the existing symbolic read and write events.
Read-only sections use the existing concrete regions.
The method splits a concrete reservation where a section overlaps it.
It rejects empty sections, address overflow, repeated sections, and overlap with other region kinds.
An error leaves the memory unchanged.

A read across initialized sections retains all its bytes.
If a read includes writable bytes, the existing symbolic-memory event carries the read.
A partial initial image causes an explicit error.
Writes that overlap read-only bytes also cause an explicit error.
A possible symbolic overlap does not remove that execution from the query.
The model does not yet translate this error into an operating-system permission fault.

The SMT initial-memory function contains the recorded bytes.
Each read assembles bytes in little-endian order, including reads through symbolic addresses.
Overlapping reads and different read widths refer to the same bytes.
Existing custom regions and legacy initial locations retain their existing fallback and override behavior.
The final-memory expression also uses the recorded initial bytes.

This change does not replace the memory model or add sequential forwarding.
It does not establish arbitrary mixed-width write overlap or full concurrent-program coverage.
Symbolic pointers into read-only regions still use the upstream concrete-overlap restriction.
The separate tag-write path remains unchanged.

The native tests cover symbolic initial reads of one, two, four, and eight bytes.
Each query is satisfiable before an incorrect-value assertion makes it unsatisfiable.
The native memory tests also cover section boundaries and read-only store errors.
