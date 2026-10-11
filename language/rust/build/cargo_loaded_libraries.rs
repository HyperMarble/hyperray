// Purpose: says which of the compiler's ready made libraries the built
//          files load at run time. Rust's standard library is one, when the
//          build left it out of the program: its code is then in no file the
//          build made. A built file carries the name of every file it loads.
// Never:   hands over a library no built file loads.
use super::cargo_compiler_libraries::shipped;
use crate::blocked::Blocked;
use crate::cargo_build::Built;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::path::{
    Path,
    PathBuf, //
};

/// What the record calls a library the compiler ships ready made, as it
/// calls nothing Cargo names.
pub const KIND: &str = "compiler library";

/// The compiler's libraries that the files in `built` load at run time, in
/// name order, each one once. There are none when every built file has the
/// standard library inside it.
pub fn loaded(
    project: &Project,
    supplied: &Arguments,
    built: &[Built],
) -> Result<Vec<PathBuf>, Blocked> {
    let shipped = shipped(project, supplied)?;
    let mut found = Vec::new();
    for file in built {
        found.extend(loaded_by(&file.path, &shipped)?);
    }
    found.sort();
    found.dedup();
    Ok(found)
}

/// The files of `shipped` whose name the bytes of `file` hold.
fn loaded_by(file: &Path, shipped: &[PathBuf]) -> Result<Vec<PathBuf>, Blocked> {
    let bytes = std::fs::read(file).map_err(|error| unreadable(file, error))?;
    let mut found = Vec::new();
    for library in shipped {
        let Some(name) = library.file_name() else {
            continue;
        };
        if holds(&bytes, name.as_encoded_bytes()) {
            found.push(library.clone());
        }
    }
    Ok(found)
}

/// True when `name` stands somewhere inside `bytes`.
fn holds(bytes: &[u8], name: &[u8]) -> bool {
    for part in bytes.windows(name.len()) {
        if part == name {
            return true;
        }
    }
    false
}

fn unreadable(file: &Path, error: std::io::Error) -> Blocked {
    Blocked::Unreadable {
        what: file.display().to_string(),
        cause: error.to_string(),
    }
}
