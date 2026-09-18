// Instance kinds preserve the compiler distinction for generated callables.
// This match must fail to compile when rustc adds an unhandled kind.

use hyperray_rust::mir::CompilerInstanceKind;
use rustc_middle::ty::InstanceKind;

pub fn of(kind: InstanceKind<'_>) -> CompilerInstanceKind {
    match kind {
        InstanceKind::Item(_) => CompilerInstanceKind::Function,
        InstanceKind::Intrinsic(_) => CompilerInstanceKind::Intrinsic,
        InstanceKind::LlvmIntrinsic(_) => CompilerInstanceKind::LlvmIntrinsic,
        InstanceKind::Virtual(_, _) => CompilerInstanceKind::Virtual,
        InstanceKind::Shim(_) => CompilerInstanceKind::Shim,
    }
}
