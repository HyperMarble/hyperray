// Purpose: a build with packed debug information is recorded, and the
//          program's debug information (the one file inside Apple's `.dSYM`
//          bundle) is recorded with it, named by its hash.
// Never:   drops debug information the build made, or blocks because of it.
mod sample_project;

use language_rust::build_record_with;
use language_rust::cargo_request::Arguments;
use language_rust::record::Outcome;
use sample_project::{
    create,
    Sample, //
};
use std::io::Write;

/// Builds a small program whose release profile packs its debug information.
fn built_with_debug_info() -> Result<Outcome, String> {
    let source = "fn main() { println!(\"{}\", std::env::args().count()); }\n";
    let sample = Sample {
        name: "debug_info",
        file: "main.rs",
        source,
        locked: true,
    };
    let root = create(&sample)?;
    let mut manifest = std::fs::OpenOptions::new()
        .append(true)
        .open(root.join("Cargo.toml"))
        .map_err(|e| e.to_string())?;
    let profile = b"\n[profile.release]\ndebug = true\nsplit-debuginfo = \"packed\"\n";
    manifest.write_all(profile).map_err(|e| e.to_string())?;
    let supplied = Arguments {
        build: vec!["--release".into()],
        ..Arguments::default()
    };
    Ok(build_record_with(&root, &supplied))
}

#[test]
fn a_build_with_debug_information_is_recorded() -> Result<(), String> {
    let outcome = built_with_debug_info()?;
    let Outcome::Built(record) = outcome else {
        return Err(format!(
            "a build with debug information was blocked: {outcome:?}"
        ));
    };
    assert!(record
        .artifacts
        .iter()
        .all(|a| !a.file.path.ends_with(".dSYM")));
    let program = record
        .artifacts
        .iter()
        .find(|a| a.kind == "bin")
        .ok_or("no program")?;
    let debug = program
        .debug_info
        .as_ref()
        .ok_or("the program's debug information was not recorded")?;
    assert!(
        debug.path.contains(".dSYM/Contents/Resources/DWARF/"),
        "{}",
        debug.path
    );
    assert_eq!(debug.sha256.len(), 64);
    Ok(())
}
