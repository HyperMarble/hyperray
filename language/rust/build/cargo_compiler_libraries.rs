// Purpose: lists the dynamic libraries the compiler ships ready made, such
//          as Rust's standard library. The compiler Cargo runs says where
//          its libraries are and what it calls a dynamic library.
// Never:   guesses the folder, or the form of a file's name.
use super::cargo_dynamic_names::dynamic_library_names;
use crate::blocked::Blocked;
use crate::cargo_compiler::answer;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::path::{
    Path,
    PathBuf, //
};

/// Every dynamic library in the compiler's library folders. There are none
/// when the chip being built for has no dynamic libraries.
pub fn shipped(project: &Project, supplied: &Arguments) -> Result<Vec<PathBuf>, Blocked> {
    let folders = answer(project, supplied, &["--print", "target-libdir"])?;
    let names = dynamic_library_names(project, supplied)?;
    let mut found = Vec::new();
    for folder in folders.lines() {
        found.extend(dynamic_libraries(Path::new(folder), &names)?);
    }
    Ok(found)
}

/// The files in `folder` that are dynamic libraries.
fn dynamic_libraries(folder: &Path, names: &[String]) -> Result<Vec<PathBuf>, Blocked> {
    let entries = std::fs::read_dir(folder).map_err(|error| unreadable(folder, error))?;
    let mut found = Vec::new();
    for entry in entries {
        let file = entry.map_err(|error| unreadable(folder, error))?.path();
        if ends_like_one_of(&file, names) {
            found.push(file);
        }
    }
    Ok(found)
}

/// True when `file` has the same ending as one of `names`, the names the
/// compiler gives a dynamic library: `libhyperray_compiler.dylib` makes
/// `libstd-3f21.dylib` one, and `libstd-3f21.rlib` not.
fn ends_like_one_of(file: &Path, names: &[String]) -> bool {
    for name in names {
        if file.extension() == Path::new(name).extension() {
            return true;
        }
    }
    false
}

fn unreadable(folder: &Path, error: std::io::Error) -> Blocked {
    Blocked::Unreadable {
        what: folder.display().to_string(),
        cause: error.to_string(),
    }
}
