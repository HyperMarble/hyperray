# Machine ELF fixtures

These fixtures came from LLVM and LLD on an Apple Silicon Mac. They are not hand-written ELF files.

The source toolchain was Rust 1.98.0 with LLVM 22.1.8 and LLD 22.1.8.

Run this command from the repository root:

```text
fixtures/machine/generate.sh
```

The canonical fixture is `rv64-lp64d-static.elf`. It contains separate read-execute and read-write load segments.

The data segment has one file byte and three zero-filled bytes. The code segment uses 16-bit and 32-bit encodings.

The other ELF files exercise class, byte order, ABI, dynamic loading, memory permissions, and instruction framing rejections.

These fixtures test ELF loading and instruction framing only. They do not test RVA20U64 instruction semantics.
