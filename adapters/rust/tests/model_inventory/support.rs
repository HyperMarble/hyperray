// Live model tests get every external path and option from their environment.
// The adapter test code therefore has no knowledge of a fixture name.

use hyperray_rust::bound::{build_model, HarnessMode, Model, Scope};
use std::path::PathBuf;

pub struct Config {
    crate_dir: PathBuf,
    output_dir: PathBuf,
    cargo: PathBuf,
    cbmc: PathBuf,
    arguments: Vec<String>,
}

pub fn configured() -> Option<Config> {
    let crate_dir = std::env::var_os("HYPERRAY_KANI_CRATE").map(PathBuf::from)?;
    let output_dir =
        std::env::temp_dir().join(format!("hyperray-model-inventory-{}", std::process::id()));
    Some(Config {
        crate_dir,
        output_dir,
        cargo: program("HYPERRAY_CARGO", "cargo"),
        cbmc: program("HYPERRAY_CBMC", "cbmc"),
        arguments: arguments(),
    })
}

pub fn build(config: &Config, mode: HarnessMode) -> Result<Model, hyperray_rust::bound::Error> {
    let scope = Scope {
        crate_dir: &config.crate_dir,
        output_dir: &config.output_dir,
        cargo: &config.cargo,
        cbmc: &config.cbmc,
        harness_mode: mode,
        kani_arguments: &config.arguments,
    };
    build_model(&[], &[], &scope)
}

fn program(variable: &str, default: &str) -> PathBuf {
    match std::env::var_os(variable) {
        Some(value) => PathBuf::from(value),
        None => PathBuf::from(default),
    }
}

fn arguments() -> Vec<String> {
    match std::env::var("HYPERRAY_KANI_ARGS") {
        Ok(value) => value.split_whitespace().map(str::to_string).collect(),
        Err(_) => Vec::new(),
    }
}
