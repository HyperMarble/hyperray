// Record concrete compiler instances and every retained raw body position.
// This mode never changes legacy extraction or invents semantic mappings.

mod body;
mod callable;
mod digest;
mod instance;
mod kind;
mod payload;
mod records;
mod root_facts;
use hyperray_rust::mir::{RawInstance, RawInventory};
use rustc_middle::ty::TyCtxt;
use std::collections::BTreeMap;
use std::fs::OpenOptions;
use std::io::Write;
use std::path::PathBuf;
const INVENTORY_VERSION: u32 = 1;
pub fn write(tcx: TyCtxt<'_>) -> Result<PathBuf, String> {
    let directory = directory()?;
    std::fs::create_dir_all(&directory).map_err(|error| error.to_string())?;
    let compilation_id = std::env::var("HYPERRAY_COMPILATION_ID")
        .map_err(|_| "missing compilation id".to_string())?;
    let target = tcx.sess.opts.target_triple.to_string();
    let inventory = RawInventory {
        version: INVENTORY_VERSION,
        compilation_id: compilation_id.clone(),
        instances: instances(tcx, &compilation_id, &target)?,
    };
    let path = directory.join("inventory.json");
    let bytes = serde_json::to_vec_pretty(&inventory).map_err(|error| error.to_string())?;
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
        .map_err(|error| error.to_string())?;
    file.write_all(&bytes).map_err(|error| error.to_string())?;
    Ok(path)
}
fn directory() -> Result<PathBuf, String> {
    std::env::var_os("HYPERRAY_INVENTORY_DIR")
        .map(PathBuf::from)
        .ok_or_else(|| "missing inventory directory".to_string())
}
fn instances(
    tcx: TyCtxt<'_>,
    compilation_id: &str,
    target: &str,
) -> Result<Vec<RawInstance>, String> {
    let partitions = tcx.collect_and_partition_mono_items(());
    let mut items = BTreeMap::new();
    for unit in partitions.codegen_units {
        for item in unit.items().keys() {
            let record = instance::from_item(tcx, *item, compilation_id, target)?;
            insert_record(&mut items, record)?;
        }
    }
    Ok(items.into_values().collect())
}
fn insert_record(
    items: &mut BTreeMap<String, RawInstance>,
    record: RawInstance,
) -> Result<(), String> {
    let Some(previous) = items.get(&record.id) else {
        items.insert(record.id.clone(), record);
        return Ok(());
    };
    if previous == &record {
        return Ok(());
    }
    Err(format!(
        "conflicting compiler inventory identity: {}",
        record.id
    ))
}
