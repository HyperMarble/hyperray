// Purpose: the adapter's own extra build never changes a file the owner's
//          build made: each one is still the file Cargo wrote.
// Never:   compares the adapter with itself: Cargo's build is the judge.
mod sample_project;

use language_rust::build_record;
use sample_project::{
    create,
    Sample, //
};
use serde_json::Value;
use sha2::{
    Digest,
    Sha256, //
};
use std::io::Write;
use std::path::{
    Path,
    PathBuf, //
};
use std::process::Command;

fn fingerprint(path: &Path) -> Result<String, String> {
    let bytes = std::fs::read(path).map_err(|error| error.to_string())?;
    Ok(format!("{:x}", Sha256::digest(&bytes)))
}

/// The dynamic library Cargo itself reports for a plain build of `root`.
fn owners_dynamic_library(root: &Path) -> Result<PathBuf, String> {
    let words = ["build", "--locked", "--offline", "--message-format=json"];
    let owner = Command::new("cargo").args(words).current_dir(root).output();
    let printed =
        String::from_utf8_lossy(&owner.map_err(|error| error.to_string())?.stdout).into_owned();
    let ending = format!(".{}\"", std::env::consts::DLL_EXTENSION);
    let line = printed
        .lines()
        .find(|line| line.contains(&ending))
        .ok_or("cargo made no dynamic library")?;
    let message: Value = serde_json::from_str(line).map_err(|error| error.to_string())?;
    let files = message["filenames"].as_array().ok_or("no filenames")?;
    let own = files
        .iter()
        .filter_map(Value::as_str)
        .find(|file| file.ends_with(ending.trim_end_matches('"')));
    own.map(PathBuf::from)
        .ok_or_else(|| "no dynamic library among the files".to_string())
}

#[test]
fn a_library_that_is_also_dynamic_keeps_its_own_file() -> Result<(), String> {
    let root = create(&Sample {
        name: "owner_file_kept",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"));
    let mut manifest = manifest.map_err(|error| error.to_string())?;
    writeln!(manifest, "\n[lib]\ncrate-type = [\"cdylib\", \"rlib\"]")
        .map_err(|error| error.to_string())?;
    let own = owners_dynamic_library(&root)?;
    let before = fingerprint(&own)?;
    let record = serde_json::to_value(build_record(&root)).map_err(|error| error.to_string())?;
    assert_eq!(record["status"], "built");
    assert_eq!(
        fingerprint(&own)?,
        before,
        "the owner's own library was overwritten"
    );
    Ok(())
}
