// Purpose: compares the adapter's default profile with Cargo's actual report.
// Never: copies Cargo profile defaults into expected values.
mod sample_project;

use language_rust::cargo_request::Arguments;
use sample_project::{
    create,
    Sample, //
};
use serde_json::Value;
use std::path::Path;
use std::process::Command;

#[test]
fn the_default_build_keeps_cargos_profile() -> Result<(), String> {
    let root = create(&Sample {
        name: "cargo_default_profile",
        file: "lib.rs",
        source: "pub fn value() -> u8 { 1 }\n",
        locked: true,
    })?;
    let owner = owner_artifacts(&root)?;
    let project = language_rust::project::find(&root).map_err(|error| error.to_string())?;
    let build = language_rust::cargo_build::build(&project, &Arguments::default())
        .map_err(|error| error.to_string())?;
    assert!(!build.files.is_empty());
    for file in &build.files {
        let path = serde_json::to_value(&file.path).map_err(|error| error.to_string())?;
        let message = owner
            .iter()
            .find(|message| {
                message["filenames"]
                    .as_array()
                    .is_some_and(|files| files.contains(&path))
            })
            .ok_or("Cargo control did not report the adapter's artifact")?;
        let profile = &message["profile"];
        assert_eq!(file.compiled.opt_level, profile["opt_level"]);
        assert_eq!(file.compiled.debug_assertions, profile["debug_assertions"]);
        assert_eq!(file.compiled.overflow_checks, profile["overflow_checks"]);
        assert_eq!(file.compiled.test, profile["test"]);
    }
    Ok(())
}

fn owner_artifacts(root: &Path) -> Result<Vec<Value>, String> {
    let output = Command::new("cargo")
        .args(["build", "--locked", "--offline", "--message-format=json"])
        .current_dir(root)
        .output()
        .map_err(|error| error.to_string())?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).to_string());
    }
    let text = String::from_utf8(output.stdout).map_err(|error| error.to_string())?;
    let mut artifacts = Vec::new();
    for line in text.lines() {
        let message: Value = serde_json::from_str(line).map_err(|error| error.to_string())?;
        if message["reason"] == "compiler-artifact" {
            artifacts.push(message);
        }
    }
    Ok(artifacts)
}
