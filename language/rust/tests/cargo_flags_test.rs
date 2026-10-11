// Purpose: every flag `cargo build --help` names has a line in the table
//          saying what the adapter does with it, and the table names no flag
//          Cargo dropped.
// Never:   trusts a hand-written flag list over Cargo's own help.
#[path = "cargo_flags/table.rs"]
mod table;

use std::collections::BTreeSet;
use std::process::Command;

/// The flags Cargo's help names, each by its long name or its only name.
fn named_by_cargo() -> Result<BTreeSet<String>, String> {
    let output = Command::new("cargo")
        .args(["build", "--help"])
        .output()
        .map_err(|error| error.to_string())?;
    let help = String::from_utf8(output.stdout).map_err(|error| error.to_string())?;
    Ok(help.lines().filter_map(flag_of).collect())
}

/// `  -p, --package [<SPEC>]  Package to build` names `--package`.
fn flag_of(line: &str) -> Option<String> {
    let mut words = line.split_whitespace();
    let first = words.next().filter(|word| word.starts_with('-'))?;
    let name = if first.ends_with(',') {
        words.next()?
    } else {
        first
    };
    Some(name.trim_end_matches("...").to_string())
}

#[test]
fn every_cargo_build_flag_has_a_handling() -> Result<(), String> {
    let named = named_by_cargo()?;
    if named.is_empty() {
        return Err("cargo build --help names no flags".to_string());
    }
    let listed: BTreeSet<String> = table::HANDLING
        .iter()
        .map(|(flag, _)| flag.to_string())
        .collect();
    let missing: Vec<&String> = named.difference(&listed).collect();
    let dropped: Vec<&String> = listed.difference(&named).collect();
    assert!(
        missing.is_empty(),
        "cargo build has {missing:?}, the table does not"
    );
    assert!(
        dropped.is_empty(),
        "the table names {dropped:?}, cargo build does not"
    );
    Ok(())
}
