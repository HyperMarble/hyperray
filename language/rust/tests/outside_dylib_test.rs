// Purpose: a dynamic library crate that comes from outside the project is
//          handed over. A program loads it at run time as a file of its own,
//          so unlike every other outside crate its code is in no other file.
// Never:   decides by hand which file that crate makes: Cargo names it.
mod sample_project;
mod vendored_crate;

use language_rust::build_record;
use sample_project::{
    create,
    Sample, //
};
use serde_json::Value;
use std::path::Path;
use std::process::Command;

fn cargo(root: &Path, words: &[&str]) -> Result<String, String> {
    let output = Command::new("cargo").args(words).current_dir(root).output();
    let output = output.map_err(|error| error.to_string())?;
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

/// The file Cargo's own build reports for the dynamic library it made.
fn owners_dynamic_library(root: &Path) -> Result<String, String> {
    let words = ["build", "--locked", "--offline", "--message-format=json"];
    let printed = cargo(root, &words)?;
    for line in printed.lines() {
        let Ok(message) = serde_json::from_str::<Value>(line) else {
            continue;
        };
        if message["target"]["kind"][0] == "dylib" {
            let file = message["filenames"][0].as_str();
            return file.map(str::to_string).ok_or("no file name".to_string());
        }
    }
    Err("cargo's own build made no dynamic library".to_string())
}

/// True when the record holds a file at `path`.
fn is_in_record(record: &Value, path: &str) -> bool {
    let Some(files) = record["artifacts"].as_array() else {
        return false;
    };
    files.iter().any(|file| file["file"]["path"] == path)
}

#[test]
fn a_dynamic_library_from_outside_the_project_is_handed_over() -> Result<(), String> {
    let root = create(&Sample {
        name: "dylib_app",
        file: "main.rs",
        source: "fn main() { println!(\"{}\", vendored_shared::one()); }\n",
        locked: false,
    })?;
    vendored_crate::add_to(&root)?;
    cargo(&root, &["generate-lockfile", "--offline"])?;
    let wanted = owners_dynamic_library(&root)?;
    let record = serde_json::to_value(build_record(&root)).map_err(|error| error.to_string())?;
    let reason = &record["reason"];
    assert!(
        is_in_record(&record, &wanted),
        "{wanted} is not in the record. {reason}"
    );
    Ok(())
}
