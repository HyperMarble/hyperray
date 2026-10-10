// Purpose: a build never rewrites the project's pinned dependency versions.
// Never:   lets a stale Cargo.lock build anyway, with versions it does not pin.
mod sample_project;

use language_rust::blocked::Blocked;
use language_rust::cargo_build::build;
use language_rust::cargo_request::Arguments;
use language_rust::project::find;
use sample_project::{
    create,
    Sample, //
};
use std::io::Write;

#[test]
fn a_stale_lock_file_blocks_the_build_and_stays_unchanged() -> Result<(), String> {
    create(&Sample {
        name: "sample_dependency",
        file: "lib.rs",
        source: "pub fn two() -> u8 { 2 }\n",
        locked: false,
    })?;
    let root = create(&Sample {
        name: "sample_stale_lock",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let lock_before = std::fs::read(root.join("Cargo.lock")).map_err(|error| error.to_string())?;
    let mut manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"))
        .map_err(|error| error.to_string())?;
    let added = "\n[dependencies]\nsample_dependency = { path = \"../sample_dependency\" }\n";
    manifest
        .write_all(added.as_bytes())
        .map_err(|error| error.to_string())?;
    let project = find(&root).map_err(|blocked| blocked.to_string())?;
    let result = build(&project, &Arguments::default());
    assert!(matches!(result, Err(Blocked::ToolFailed { .. })));
    let lock_after = std::fs::read(root.join("Cargo.lock")).map_err(|error| error.to_string())?;
    assert_eq!(lock_before, lock_after);
    Ok(())
}
