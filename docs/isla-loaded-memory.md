# Loaded memory coverage

The executable loader records each byte and its access permissions.
Program generation must preserve both values and writable regions.
A writable region must not become a constant during symbolic execution.

The pinned Isla section loader treats every section as read-only memory.
This behavior does not represent a writable ELF segment.
The integration will reuse Isla's symbolic memory events and memory model.
The solver's initial-memory relation must retain the loaded bytes.
Each read assembles bytes in the declared little-endian target order.
The relation must support symbolic addresses, overlaps, and different access sizes.

The dependency extension adds a writable attribute to sections.
Writable sections use symbolic reads and writes, with recorded initial bytes.
Read-only sections keep their loaded bytes in the initial-memory relation too.
The finite executable image supplies the bytes and permissions, not source patterns.
Unknown tool support must stop verification before a verdict.

The first regression stores an input in a Rust atomic and reads it again.
Its expected result follows from the stored value and the selected memory model.
Separate regressions cover initial bytes and overlapping accesses.
These tests do not establish operating-system, thread, or full Rust coverage.
