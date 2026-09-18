// The compiler codegen partition supplies the exact monomorphized item catalog.
// This module never invents instances from source-level generic definitions.

use crate::instance_kind;
use hyperray_rust::mir::{CompilerInstance, CompilerInstanceKind};
use rustc_middle::mono::MonoItem;
use rustc_middle::ty::{TyCtxt, TypingEnv};
use std::collections::BTreeMap;

pub fn of(tcx: TyCtxt<'_>) -> Result<Vec<CompilerInstance>, String> {
    let partitions = tcx.collect_and_partition_mono_items(());
    let mut instances = BTreeMap::new();
    for unit in partitions.codegen_units {
        for item in unit.items().keys() {
            insert(&mut instances, one(tcx, *item))?;
        }
    }
    Ok(instances.into_values().collect())
}

fn insert(
    instances: &mut BTreeMap<String, CompilerInstance>,
    instance: CompilerInstance,
) -> Result<(), String> {
    if let Some(previous) = instances.get(&instance.id) {
        if previous != &instance {
            return Err(format!(
                "compiler instance {} has conflicting records",
                instance.id
            ));
        }
        return Ok(());
    }
    instances.insert(instance.id.clone(), instance);
    Ok(())
}

fn one<'tcx>(tcx: TyCtxt<'tcx>, item: MonoItem<'tcx>) -> CompilerInstance {
    let symbol = item.symbol_name(tcx).to_string();
    match item {
        MonoItem::Fn(instance) => CompilerInstance {
            id: symbol.clone(),
            symbol,
            name: tcx.def_path_str(instance.def_id()),
            kind: instance_kind::of(instance.def),
            generic_arguments: instance
                .args
                .iter()
                .map(|value| format!("{value:?}"))
                .collect(),
            type_signature: format!("{:?}", instance.ty(tcx, TypingEnv::fully_monomorphized())),
        },
        MonoItem::Static(definition) => CompilerInstance {
            id: symbol.clone(),
            symbol,
            name: tcx.def_path_str(definition),
            kind: CompilerInstanceKind::Static,
            generic_arguments: Vec::new(),
            type_signature: format!("{:?}", tcx.type_of(definition).instantiate_identity()),
        },
        MonoItem::GlobalAsm(item) => CompilerInstance {
            id: symbol.clone(),
            symbol,
            name: format!("{item:?}"),
            kind: CompilerInstanceKind::GlobalAssembly,
            generic_arguments: Vec::new(),
            type_signature: "global_assembly".to_string(),
        },
    }
}
