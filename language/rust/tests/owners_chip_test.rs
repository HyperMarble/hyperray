// Purpose: the adapter's question to the compiler is asked for the chip the
//          owner builds for, however the owner writes the flag.
// Never:   decides by hand where a chip's libraries are: the compiler says.
mod sample_project;

use language_rust::cargo_compiler::answer;
use language_rust::cargo_request::Arguments;
use sample_project::{
    create,
    Sample, //
};
use std::path::Path;
use std::process::Command;

/// What the compiler itself prints when asked `words` from `root`.
fn compiler_says(root: &Path, words: &[&str]) -> Result<String, String> {
    let output = Command::new("rustc").args(words).current_dir(root).output();
    let output = output.map_err(|error| error.to_string())?;
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

/// A chip the compiler knows that is not the one it runs on.
fn another_chip(root: &Path) -> Result<String, String> {
    let host = compiler_says(root, &["--print", "host-tuple"])?;
    let chips = compiler_says(root, &["--print", "target-list"])?;
    for chip in chips.lines() {
        if chip != host.trim() {
            return Ok(chip.to_string());
        }
    }
    Err("the compiler knows no other chip".to_string())
}

/// Where the adapter's question says the compiler's libraries are, when the
/// owner's build flags are `flags`.
fn adapter_says(root: &Path, flags: &[&str]) -> Result<String, String> {
    let project = language_rust::project::find(root).map_err(|blocked| blocked.to_string())?;
    let supplied = Arguments {
        build: flags.iter().map(Into::into).collect(),
        ..Arguments::default()
    };
    let said = answer(&project, &supplied, &["--print", "target-libdir"]);
    said.map_err(|blocked| blocked.to_string())
}

#[test]
fn the_compiler_is_asked_about_the_chip_the_owner_names() -> Result<(), String> {
    let root = create(&Sample {
        name: "named_chip",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let chip = another_chip(&root)?;
    let wanted = compiler_says(&root, &["--target", &chip, "--print", "target-libdir"])?;
    assert_eq!(adapter_says(&root, &["--target", &chip])?, wanted);
    let joined = format!("--target={chip}");
    assert_eq!(adapter_says(&root, &[&joined])?, wanted);
    Ok(())
}
