// One unique JSON file for each compiler process. Existing output is never
// overwritten, so stale data cannot appear as fresh data.

use hyperray_rust::mir::Dump;
use std::fs::OpenOptions;
use std::io::Write;
use std::path::PathBuf;

pub fn write(dump: &Dump) -> Result<PathBuf, String> {
    let directory = directory();
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("cannot create {}: {error}", directory.display()))?;
    let name = format!("{}-{}.mir.json", dump.crate_name, std::process::id());
    let path = directory.join(name);
    let bytes = serde_json::to_vec(dump).map_err(|error| error.to_string())?;
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
        .map_err(|error| format!("cannot create {}: {error}", path.display()))?;
    file.write_all(&bytes)
        .map_err(|error| format!("cannot write {}: {error}", path.display()))?;
    Ok(path)
}

fn directory() -> PathBuf {
    std::env::var("HYPERRAY_MIR_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|_| PathBuf::from("target/hyperray-mir"))
}
