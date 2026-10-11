// Purpose: the compiler in the record is the one Cargo ran, also when the
//          owner tells Cargo to use another compiler.
// Never:   decides by hand which compiler that is: Cargo's own build names it.
mod sample_project;
mod second_compiler;

use language_rust::cargo_request::Arguments;
use sample_project::{
    create,
    Sample, //
};
use std::path::Path;
use std::process::Command;

/// What Cargo prints in front of each compiler command it runs.
const RUNNING: &str = "Running `";

/// The compiler Cargo's own build says it ran when given `setting`. The
/// build goes to a folder of its own, so Cargo always compiles and says so.
fn compiler_cargo_ran(root: &Path, setting: &str) -> Result<String, String> {
    let words = ["build", "--locked", "--offline", "-v", "--config", setting];
    let mut cargo = Command::new("cargo");
    cargo.args(words).args(["--target-dir", "judge"]);
    let output = cargo.current_dir(root).output();
    let output = output.map_err(|error| error.to_string())?;
    let said = String::from_utf8_lossy(&output.stderr).into_owned();
    for line in said.lines() {
        if let Some(command) = line.trim().strip_prefix(RUNNING) {
            let program = command.split(' ').next();
            return Ok(program.unwrap_or_default().to_string());
        }
    }
    Err(said)
}

#[test]
fn the_compiler_the_owner_names_is_the_one_recorded() -> Result<(), String> {
    let root = create(&Sample {
        name: "named_compiler",
        file: "main.rs",
        source: "fn main() {}\n",
        locked: true,
    })?;
    let program = second_compiler::install(&root.join("second"))?;
    let setting = format!("build.rustc={:?}", program.display().to_string());
    let wanted = compiler_cargo_ran(&root, &setting)?;
    let supplied = Arguments {
        build: vec!["--config".into(), setting.into()],
        ..Arguments::default()
    };
    let record = serde_json::to_value(language_rust::build_record_with(&root, &supplied));
    let record = record.map_err(|error| error.to_string())?;
    let recorded = &record["toolchain"]["compiler"]["path"];
    assert_eq!(*recorded, wanted.as_str(), "{}", record["reason"]);
    Ok(())
}
