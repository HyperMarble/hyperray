// Purpose: compares invalid configuration capture with actual Cargo refusals.
// Never: supplies handwritten compiler diagnostics as expected results.
mod sample_project;

use language_rust::blocked::Blocked;
use language_rust::cargo_request::Arguments;
use sample_project::{
    create,
    Sample, //
};
use std::path::Path;
use std::process::Command;

const MISSING: &[(&str, &str)] = &[("config.toml", "include = [\"absent.toml\"]\n")];
const MALFORMED: &[(&str, &str)] = &[("config.toml", "include = [\n")];
const CYCLE: &[(&str, &str)] = &[
    ("config.toml", "include = [\"cycle.toml\"]\n"),
    ("cycle.toml", "include = [\"config.toml\"]\n"),
];
const EXTENSION: &[(&str, &str)] = &[
    ("config.toml", "include = [\"settings.txt\"]\n"),
    (
        "settings.txt",
        "[profile.release]\noverflow-checks = true\n",
    ),
];

#[test]
fn configuration_refusals_preserve_the_owner_trace() -> Result<(), String> {
    let cases = [
        ("required_missing", MISSING),
        ("malformed", MALFORMED),
        ("cycle", CYCLE),
        ("extension", EXTENSION),
    ];
    for (name, files) in cases {
        let name = format!("config_refusal_{name}");
        let root = create(&Sample {
            name: &name,
            file: "lib.rs",
            source: "pub fn value() -> u8 { 1 }\n",
            locked: true,
        })?;
        std::fs::create_dir(root.join(".cargo")).map_err(|error| error.to_string())?;
        for (name, contents) in files {
            std::fs::write(root.join(".cargo").join(name), contents)
                .map_err(|error| error.to_string())?;
        }
        compare(&root)?;
    }
    Ok(())
}

fn compare(root: &Path) -> Result<(), String> {
    let owner = Command::new("cargo")
        .args(["metadata", "--no-deps", "--locked", "--format-version", "1"])
        .current_dir(root)
        .output()
        .map_err(|error| error.to_string())?;
    let captured = language_rust::config_files::report_for_request(root, &[]);
    assert_eq!(captured.is_ok(), owner.status.success(), "{captured:?}");
    let built = language_rust::project::find(root)
        .and_then(|project| language_rust::cargo_build::build(&project, &Arguments::default()));
    match (built, owner.status.success()) {
        (Err(Blocked::ToolFailed { output, .. }), false) => {
            assert_eq!(output.as_bytes(), owner.stderr);
        }
        (Ok(_), true) => {}
        (observed, success) => {
            return Err(format!("Cargo success={success}; adapter={observed:?}"))
        }
    }
    Ok(())
}
