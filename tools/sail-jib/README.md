# Sail JIB constructor census

This tool uses the pinned Sail C backend to lower the typed Sail model to JIB.
It records each JIB constructor origin and the kinds in the complete model.

The origin identifiers use deterministic traversal order. They do not contain
an instruction name, source-function name, address, byte value, or bound.

The tool does not contain RISC-V instruction names or instruction behavior.
It classifies only the fixed JIB grammar.

The shell command copies the pinned upstream `c_backend.ml` into a temporary
build directory. Thus, Hyperray does not copy or replace the Sail translator.

Operate the reproducible RV64 base-I census from the Hyperray root:

```sh
./tools/sail-jib/run.sh /path/to/sail-riscv /path/to/sail
```

A completed report means that Sail lowered the full accepted profile and that
the census visited its JIB tree. It does not mean that Hyperray can translate
all recorded JIB kinds to circuits.
