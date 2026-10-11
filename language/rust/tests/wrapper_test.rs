// Purpose: a library Cargo will not link by itself, one without the standard
//          library, is still handed over finished, through a small crate of
//          the adapter's own that only uses it.
// Never:   uses that crate when the project's own Cargo.toml holds build
//          settings it would not get.
mod library_files;
mod sample_project;

use language_rust::build_record;
use library_files::{
    file_of_kind,
    functions,
    record_of, //
};
use sample_project::{
    create,
    Sample, //
};
use std::io::Write;

const WITHOUT_STD: &str = "#![no_std]\n\npub fn total(each: u64, count: u64) -> u64 {\n    \
helper(each) * count\n}\n\nfn helper(x: u64) -> u64 {\n    x + 1\n}\n";

#[test]
fn a_library_without_the_standard_library_is_finished() -> Result<(), String> {
    let record = record_of("wrapped_whole", WITHOUT_STD)?;
    let library = file_of_kind(&record, "lib").ok_or("the library file is not in the record")?;
    let finished = file_of_kind(&record, "finished library");
    let finished = finished.ok_or_else(|| record["not_built"].to_string())?;
    let wanted = functions(&library, "wrapped_whole")?;
    let found = functions(&finished, "wrapped_whole")?;
    assert!(wanted.len() >= 2, "nm found {wanted:?} in {library}");
    let missing: Vec<&String> = wanted.difference(&found).collect();
    assert!(
        missing.is_empty(),
        "missing from the finished library: {missing:?}"
    );
    Ok(())
}

#[test]
fn a_project_with_its_own_build_settings_is_not_wrapped() -> Result<(), String> {
    let root = create(&Sample {
        name: "wrapped_settings",
        file: "lib.rs",
        source: WITHOUT_STD,
        locked: true,
    })?;
    let manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"));
    let mut manifest = manifest.map_err(|error| error.to_string())?;
    writeln!(manifest, "\n[profile.dev]\nopt-level = 1").map_err(|error| error.to_string())?;
    let record = serde_json::to_value(build_record(&root)).map_err(|error| error.to_string())?;
    assert!(file_of_kind(&record, "finished library").is_none());
    let reason = record["not_built"][0]["reason"]
        .as_str()
        .unwrap_or_default();
    assert!(
        reason.contains("panic_handler") && reason.contains("not added yet"),
        "{reason}"
    );
    Ok(())
}
