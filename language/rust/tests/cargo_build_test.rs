// Purpose: the project's own release build runs and its files are listed.
// Never:   reports a failed build as built, or lists metadata-only files.
mod sample_project;

use language_rust::blocked::Blocked;
use language_rust::cargo_build::{build, Built};
use language_rust::project::find;
use sample_project::{create, Sample};

fn built(name: &str, file: &str, source: &str) -> Result<Result<Vec<Built>, Blocked>, String> {
    let root = create(&Sample {
        name,
        file,
        source,
        locked: true,
    })?;
    let project = find(&root).map_err(|blocked| blocked.to_string())?;
    Ok(build(&project).map(|output| output.files))
}

#[test]
fn a_library_build_lists_its_compiled_library() -> Result<(), String> {
    let files = built("sample_lib", "lib.rs", "pub fn one() -> u8 { 1 }\n")?
        .map_err(|blocked| blocked.to_string())?;
    assert!(files
        .iter()
        .any(|file| file.kind == "lib" && file.path.is_file()));
    assert!(files
        .iter()
        .all(|file| !file.path.to_string_lossy().ends_with(".rmeta")));
    Ok(())
}

#[test]
fn a_binary_build_lists_its_executable() -> Result<(), String> {
    let files =
        built("sample_bin", "main.rs", "fn main() {}\n")?.map_err(|blocked| blocked.to_string())?;
    assert!(files
        .iter()
        .any(|file| file.kind == "bin" && file.path.is_file()));
    Ok(())
}

#[test]
fn a_build_that_fails_is_blocked_by_cargo() -> Result<(), String> {
    let result = built("sample_broken", "lib.rs", "pub fn broken( {\n")?;
    assert!(matches!(result, Err(Blocked::ToolFailed { ref tool, .. }) if tool == "cargo"));
    Ok(())
}
