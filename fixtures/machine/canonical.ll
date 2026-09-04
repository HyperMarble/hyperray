; This source produces a static RV64 LP64D ELF fixture.
; It must keep file-backed data and zero-filled data in loadable memory.
target triple = "riscv64-unknown-linux-gnu"

@loaded_byte = global i8 42, align 1
@zero_bytes = global [3 x i8] zeroinitializer, align 1

define dso_local void @_start() nounwind {
entry:
  store volatile i8 7, ptr @loaded_byte
  ret void
}
