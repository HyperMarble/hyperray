// Purpose: a library is still handed over finished when the owner's flags
//          pick what to build, and the list of such flags is Cargo's own.
// Never:   trusts a hand-written flag list over Cargo's own help.
mod sample_project;

use language_rust::cargo_request::{
    Arguments,
    PICKS, //
};
use sample_project::{
    create,
    Sample, //
};
use std::collections::BTreeSet;
use std::process::Command;

/// Every flag name Cargo's help lists under its two selection headings.
fn picks_by_cargo() -> Result<BTreeSet<String>, String> {
    let output = Command::new("cargo").args(["build", "--help"]).output();
    let printed = output.map_err(|error| error.to_string())?.stdout;
    let help = String::from_utf8_lossy(&printed).into_owned();
    let mut found = BTreeSet::new();
    let mut selecting = false;
    for line in help.lines() {
        if !line.starts_with(' ') {
            selecting = line == "Package Selection:" || line == "Target Selection:";
        } else if selecting {
            let flags = line
                .split_whitespace()
                .take_while(|word| word.starts_with('-'));
            found.extend(flags.map(|word| word.trim_end_matches(',').to_string()));
        }
    }
    Ok(found)
}

#[test]
fn the_picking_flags_are_the_ones_cargo_lists() -> Result<(), String> {
    let mut wanted = picks_by_cargo()?;
    wanted.insert("--target-dir".to_string());
    let listed: BTreeSet<String> = PICKS.iter().map(|(flag, _)| flag.to_string()).collect();
    assert_eq!(listed, wanted);
    Ok(())
}

fn finished_with(name: &str, flags: &[&str]) -> Result<bool, String> {
    let root = create(&Sample {
        name,
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    std::fs::write(root.join("src/main.rs"), "fn main() {}\n")
        .map_err(|error| error.to_string())?;
    let supplied = Arguments {
        build: flags.iter().map(Into::into).collect(),
        ..Arguments::default()
    };
    let record = serde_json::to_value(language_rust::build_record_with(&root, &supplied));
    let record = record.map_err(|error| error.to_string())?;
    let files = record["artifacts"]
        .as_array()
        .ok_or(record["reason"].to_string())?;
    Ok(files
        .iter()
        .any(|file| file["kind"][0] == "finished library"))
}

#[test]
fn a_library_is_finished_when_the_owner_picks_a_program() -> Result<(), String> {
    assert!(finished_with("picks_bin", &["--bin", "picks_bin"])?);
    assert!(finished_with(
        "picks_bin_joined",
        &["--bin=picks_bin_joined"]
    )?);
    Ok(())
}

#[test]
fn a_library_is_finished_when_the_owner_picks_the_library_or_a_package() -> Result<(), String> {
    assert!(finished_with("picks_lib", &["--lib"])?);
    assert!(finished_with("picks_package", &["-p", "picks_package"])?);
    Ok(())
}
