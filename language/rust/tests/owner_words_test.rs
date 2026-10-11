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
fn a_compile_error_reaches_the_reason_in_cargos_words() -> Result<(), String> {
    let root = create(&Sample {
        name: "owner_words_error",
        file: "lib.rs",
        source: "pub fn bad() -> u32 { \"no\" }\n",
        locked: true,
    })?;
    let owner = Command::new("cargo")
        .args(["build", "--locked", "--offline"])
        .current_dir(&root)
        .output()
        .map_err(|error| error.to_string())?;
    let shown = String::from_utf8_lossy(&owner.stderr).into_owned();
    let Outcome::Blocked { reason } = language_rust::build_record(&root) else {
        return Err("a build Cargo refuses was recorded as built".to_string());
    };
    for line in shown.lines().filter(|line| line.starts_with("error")) {
        assert!(reason.contains(line), "missing from the reason: {line}");
    }
    Ok(())
}

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
