// Convert each concrete mono item through the pinned public compiler bridge.

use super::callable::callable;
use super::records::without_body;
use hyperray_rust::mir::RawInstance;
use rustc_middle::mono::MonoItem;
use rustc_middle::ty::TyCtxt;
use rustc_public::rustc_internal;

pub(super) fn from_item<'tcx>(
    tcx: TyCtxt<'tcx>,
    item: MonoItem<'tcx>,
    compilation_id: &str,
    target: &str,
) -> Result<RawInstance, String> {
    let symbol = item.symbol_name(tcx).to_string();
    match item {
        MonoItem::Fn(internal) => callable(
            rustc_internal::stable(internal),
            compilation_id,
            symbol,
            target,
        ),
        MonoItem::Static(definition) => Ok(without_body(
            compilation_id,
            symbol.clone(),
            format!("static:{definition:?}"),
            "static",
        )),
        MonoItem::GlobalAsm(item) => Ok(without_body(
            compilation_id,
            symbol,
            format!("global_asm:{item:?}"),
            "global_assembly",
        )),
    }
}
