// Purpose: where Cargo itself builds or refuses, the adapter does the same,
//          and a refusal carries the error lines Cargo shows its owner.
// Never:   writes down by hand what Cargo is expected to print.
mod sample_project;

use language_rust::cargo_request::Arguments;
use language_rust::record::Outcome;
use sample_project::{
    create,
    Sample, //
};
use std::process::Command;

#[test]
fn a_long_config_value_cargo_accepts_is_built() -> Result<(), String> {
    let root = create(&Sample {
        name: "owner_words_long_config",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let value = format!("env.LONG=\"{}\"", "a".repeat(300));
    let owner = Command::new("cargo")
        .args(["build", "--locked", "--offline", "--config", &value])
        .current_dir(&root)
        .status()
        .map_err(|error| error.to_string())?;
    let supplied = Arguments {
        build: vec!["--config".into(), value.into()],
        ..Arguments::default()
    };
    let outcome = language_rust::build_record_with(&root, &supplied);
    assert_eq!(matches!(outcome, Outcome::Built(_)), owner.success());
    Ok(())
}
