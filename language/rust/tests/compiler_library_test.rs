// Purpose: a library the compiler ships ready made, such as Rust's standard
//          library, is handed over when a built file loads it at run time,
//          and only then. Its code is in no other file of the build.
// Never:   decides by hand what a built file loads: `otool` and the compiler
//          say.
mod loaded_files;
mod sample_project;
mod vendored_crate;

use language_rust::build_record;
use sample_project::{
    create,
    Sample, //
};
use std::collections::BTreeSet;
use std::path::{
    Path,
    PathBuf, //
};
use std::process::Command;

/// Cargo settings that make a program load the standard library at run time.
const LOADED_AT_RUN_TIME: &str = "[build]\nrustflags = [\"-C\", \"prefer-dynamic\"]\n";

const USES_THE_DYLIB: &str = "fn main() { println!(\"{}\", vendored_shared::one()); }\n";

fn program(name: &str, source: &str, locked: bool) -> Result<PathBuf, String> {
    create(&Sample {
        name,
        file: "main.rs",
        source,
        locked,
    })
}

/// Builds the project and returns the compiler's libraries its record hands
/// over, once they are the same ones the owners say its built files load.
fn handed_over(root: &Path) -> Result<BTreeSet<String>, String> {
    let record = serde_json::to_value(build_record(root)).map_err(|error| error.to_string())?;
    assert_eq!(record["status"], "built", "{}", record["reason"]);
    let handed_over = loaded_files::handed_over(&record);
    let wanted = loaded_files::owners_answer(root, &record)?;
    assert_eq!(handed_over, wanted);
    Ok(handed_over)
}

#[test]
fn a_standard_library_inside_the_program_is_not_handed_over() -> Result<(), String> {
    let root = program("std_inside", "fn main() { println!(\"hi\"); }\n", true)?;
    assert!(handed_over(&root)?.is_empty());
    Ok(())
}

#[test]
fn a_standard_library_the_program_loads_is_handed_over() -> Result<(), String> {
    let root = program("std_loaded", "fn main() { println!(\"hi\"); }\n", true)?;
    std::fs::create_dir(root.join(".cargo")).map_err(|error| error.to_string())?;
    let settings = root.join(".cargo/config.toml");
    std::fs::write(settings, LOADED_AT_RUN_TIME).map_err(|error| error.to_string())?;
    assert_eq!(handed_over(&root)?.len(), 1);
    Ok(())
}

#[test]
fn a_standard_library_two_files_load_is_handed_over_once() -> Result<(), String> {
    let root = program("std_loaded_twice", USES_THE_DYLIB, false)?;
    vendored_crate::add_to(&root)?;
    let lock = Command::new("cargo")
        .args(["generate-lockfile", "--offline"])
        .current_dir(&root)
        .status();
    assert!(lock.map_err(|error| error.to_string())?.success());
    assert_eq!(handed_over(&root)?.len(), 1);
    Ok(())
}
