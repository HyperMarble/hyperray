// Purpose: a file is labelled with every kind Cargo reports for its target.
// Never:   gives one kind to a file from a target that has several.
mod sample_project;

use language_rust::build_record;
use language_rust::record::Outcome;
use sample_project::{create, Sample};
use serde_json::json;
use std::io::Write;

#[test]
fn a_target_with_two_kinds_keeps_both_on_each_file() -> Result<(), String> {
    let root = create(&Sample {
        name: "two_kinds",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let mut manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"))
        .map_err(|error| error.to_string())?;
    writeln!(manifest, "\n[lib]\ncrate-type = [\"cdylib\", \"rlib\"]")
        .map_err(|error| error.to_string())?;
    let Outcome::Built(record) = build_record(&root) else {
        return Err("the two-kind library was not built".to_string());
    };
    let record = serde_json::to_value(&record).map_err(|error| error.to_string())?;
    let artifacts = record["artifacts"].as_array().ok_or("no artifacts")?;
    let finished = json!(["finished library"]);
    let owners: Vec<_> = artifacts
        .iter()
        .filter(|file| file["kind"] != finished)
        .collect();
    assert_eq!(owners.len(), 2);
    for artifact in owners {
        assert_eq!(artifact["kind"], json!(["cdylib", "rlib"]));
    }
    Ok(())
}

#[test]
fn a_build_script_is_not_called_a_finished_library() -> Result<(), String> {
    let root = create(&Sample {
        name: "finished_with_script",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    std::fs::write(root.join("build.rs"), "fn main() {}\n").map_err(|error| error.to_string())?;
    let Outcome::Built(record) = build_record(&root) else {
        return Err("the library with a build script was not built".to_string());
    };
    let record = serde_json::to_value(&record).map_err(|error| error.to_string())?;
    let artifacts = record["artifacts"].as_array().ok_or("no artifacts")?;
    let finished = artifacts
        .iter()
        .filter(|file| file["kind"] == json!(["finished library"]));
    let names: Vec<&str> = finished
        .filter_map(|file| file["file"]["path"].as_str())
        .collect();
    assert_eq!(names.len(), 1, "{names:?}");
    assert!(!names[0].contains("build-script"), "{names:?}");
    Ok(())
}
