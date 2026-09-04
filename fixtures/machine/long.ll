; This source produces an encoding longer than the RVA20 instruction forms.
; It must remain data so LLVM does not replace the bytes.
target triple = "riscv64-unknown-linux-gnu"

@_start = dso_local constant [2 x i8] c"\1f\00", section ".text", align 2
