// Purpose: a crate the project depends on by path, in a folder outside its
//          workspace, is handed over like the project's own crates.
// Never:   decides by hand which file the dependency makes: Cargo names it.
mod sample_project;

use language_rust::build_record;
use sample_project::{
    create,
    Sample, //
};
use serde_json::Value;
use std::io::Write;
use std::path::Path;
use std::process::Command;

fn cargo(root: &Path, words: &[&str]) -> Result<String, String> {
    let output = Command::new("cargo").args(words).current_dir(root).output();
    let output = output.map_err(|error| error.to_string())?;
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

/// The library file Cargo's own build reports for the crate called `name`.
fn owners_file(root: &Path, name: &str) -> Result<String, String> {
    let printed = cargo(
        root,
        &["build", "--locked", "--offline", "--message-format=json"],
    )?;
    let messages = printed
        .lines()
        .filter_map(|line| serde_json::from_str::<Value>(line).ok());
    let mut of_crate = messages.filter(|message| message["target"]["name"] == name);
    let message = of_crate.next().ok_or("cargo reported no such crate")?;
    let files = message["filenames"].as_array().ok_or("no filenames")?;
    let library = files
        .iter()
        .filter_map(Value::as_str)
        .find(|file| file.ends_with(".rlib"));
    library
        .map(str::to_string)
        .ok_or_else(|| "no library file".to_string())
}

#[test]
fn a_dependency_in_a_folder_outside_the_workspace_is_handed_over() -> Result<(), String> {
    create(&Sample {
        name: "outside_shared",
        file: "lib.rs",
        source: "pub fn two() -> u8 { 2 }\n",
        locked: false,
    })?;
    let root = create(&Sample {
        name: "outside_app",
        file: "main.rs",
        source: "fn main() { println!(\"{}\", outside_shared::two()); }\n",
        locked: false,
    })?;
    let manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"));
    let mut manifest = manifest.map_err(|error| error.to_string())?;
    let added = "\n[dependencies]\noutside_shared = { path = \"../outside_shared\" }\n";
    manifest
        .write_all(added.as_bytes())
        .map_err(|error| error.to_string())?;
    cargo(&root, &["generate-lockfile", "--offline"])?;
    let wanted = owners_file(&root, "outside_shared")?;
    let record = serde_json::to_value(build_record(&root)).map_err(|error| error.to_string())?;
    let files = record["artifacts"]
        .as_array()
        .ok_or(record["reason"].to_string())?;
    assert!(
        files
            .iter()
            .any(|file| file["file"]["path"] == wanted.as_str()),
        "{wanted} is not in the record"
    );
    let finished = files
        .iter()
        .filter(|file| file["kind"][0] == "finished library");
    assert_eq!(finished.count(), 1);
    Ok(())
}
