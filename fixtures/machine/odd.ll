; This source produces an executable region with one incomplete parcel.
; It must remain data so LLVM does not replace the byte.
target triple = "riscv64-unknown-linux-gnu"

@_start = dso_local constant [1 x i8] zeroinitializer, section ".text", align 1
