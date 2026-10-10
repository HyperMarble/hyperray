// Purpose: builds generated projects with owner features and profiles.
// Never: infers a feature or profile from the resulting filename.
use super::sample_project::{
    create,
    Sample, //
};
use language_rust::build_record_with;
use language_rust::cargo_request::Arguments;
use language_rust::record::{
    BuildRecord,
    Outcome, //
};
use std::io::Write;

const SOURCE: &str = r#"#[cfg(feature = "fancy")]
#[inline(never)]
fn fancy_total(number: u64) -> u64 {
    number * 3 + 1
}

fn main() {
    #[cfg(feature = "fancy")]
    println!("{}", fancy_total(std::hint::black_box(5)));
}
"#;

const MANIFEST_SETTINGS: &str = r#"
[features]
fancy = []

[profile.dist]
inherits = "release"
overflow-checks = true
"#;

pub fn build(name: &str, supplied: Arguments) -> Result<Box<BuildRecord>, String> {
    let root = create(&Sample {
        name,
        file: "main.rs",
        source: SOURCE,
        locked: true,
    })?;
    append_manifest_settings(&root)?;
    match build_record_with(&root, &supplied) {
        Outcome::Built(record) => Ok(record),
        Outcome::Blocked { reason, .. } => Err(reason),
    }
}

fn append_manifest_settings(root: &std::path::Path) -> Result<(), String> {
    let mut manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"))
        .map_err(|error| error.to_string())?;
    manifest
        .write_all(MANIFEST_SETTINGS.as_bytes())
        .map_err(|error| error.to_string())
}

pub fn contains_fancy_function(record: &BuildRecord) -> Result<bool, String> {
    let program = record
        .artifacts
        .iter()
        .find(|artifact| artifact.kind == "bin")
        .ok_or("no program")?;
    let bytes = std::fs::read(&program.file.path).map_err(|error| error.to_string())?;
    Ok(bytes.windows(11).any(|window| window == b"fancy_total"))
}
