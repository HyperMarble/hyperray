// Purpose: one call turns a project into a complete, checkable build record.
// Never:   returns a record whose hashes do not match the files on disk.
mod sample_project;

use language_rust::build_record;
use language_rust::record::Outcome;
use sample_project::{create, Sample};
use sha2::{Digest, Sha256};
use std::path::Path;

fn sha256_of(path: &str) -> Result<String, String> {
    let bytes = std::fs::read(path).map_err(|error| error.to_string())?;
    Ok(format!("{:x}", Sha256::digest(&bytes)))
}

#[test]
fn a_project_becomes_a_record_whose_hashes_match_its_files() -> Result<(), String> {
    let root = create(&Sample {
        name: "sample_record",
        file: "lib.rs",
        source: "pub fn add(a: u64, b: u64) -> u64 { a.wrapping_add(b) }\n",
        locked: true,
    })?;
    let Outcome::Built(record) = build_record(&root) else {
        return Err("the sample project was not built".to_string());
    };
    assert_eq!(record.language, "rust");
    assert!(!record.artifacts.is_empty());
    for artifact in &record.artifacts {
        assert_eq!(artifact.file.sha256, sha256_of(&artifact.file.path)?);
    }
    let lock_file = &record.settings.lock_file;
    assert_eq!(lock_file.sha256, sha256_of(&lock_file.path)?);
    assert!(!record.os_build.is_empty());
    Ok(())
}

#[test]
fn a_folder_that_is_not_a_project_gives_a_readable_reason() -> Result<(), String> {
    let outcome = build_record(Path::new(env!("CARGO_TARGET_TMPDIR")));
    let Outcome::Blocked { reason } = outcome else {
        return Err("a folder without Cargo.toml was reported as built".to_string());
    };
    assert!(reason.contains("no Cargo.toml"));
    Ok(())
}
