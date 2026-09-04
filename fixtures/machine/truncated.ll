; This source produces an executable region with one truncated 32-bit encoding.
; It must remain data so LLVM does not replace the bytes.
target triple = "riscv64-unknown-linux-gnu"

@_start = dso_local constant [2 x i8] c"\03\00", section ".text", align 2
