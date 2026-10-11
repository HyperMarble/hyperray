// Purpose: asks the compiler Cargo runs what it calls a dynamic library on
//          the chip being built for, by the file names it gives the
//          adapter's own crate.
// Never:   writes a file ending by hand: `.dylib`, `.so` and `.dll` all come
//          from the compiler.
use crate::blocked::Blocked;
use crate::cargo_compiler::answer;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::path::Path;

/// The file names the compiler gives the adapter's crate as a dynamic
/// library. The compiler is asked twice: for the crate as a plain library,
/// and for the crate as both a plain and a dynamic one. A name whose ending
/// the first answer does not have is the dynamic library's. A chip without
/// dynamic libraries gives none.
pub fn dynamic_library_names(
    project: &Project,
    supplied: &Arguments,
) -> Result<Vec<String>, Blocked> {
    let plain = answer(project, supplied, &["--print", "file-names"])?;
    let words = ["--crate-type", "dylib", "--print", "file-names"];
    let both = answer(project, supplied, &words)?;
    let mut names = Vec::new();
    for name in both.lines() {
        if !ends_like_one_of(name, &plain) {
            names.push(name.to_string());
        }
    }
    Ok(names)
}

/// True when `name` has the same ending as one of the names in `others`.
fn ends_like_one_of(name: &str, others: &str) -> bool {
    let ending = Path::new(name).extension();
    others
        .lines()
        .any(|other| Path::new(other).extension() == ending)
}
