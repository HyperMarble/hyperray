// Preserve the compiler's public instance kind without semantic interpretation.

use rustc_public::mir::mono::InstanceKind;

pub(super) fn name(kind: InstanceKind) -> String {
    match kind {
        InstanceKind::Item => "function".to_string(),
        InstanceKind::Intrinsic => "intrinsic".to_string(),
        InstanceKind::LlvmIntrinsic => "llvm_intrinsic".to_string(),
        InstanceKind::Virtual { .. } => "virtual".to_string(),
        InstanceKind::Shim => "shim".to_string(),
    }
}
