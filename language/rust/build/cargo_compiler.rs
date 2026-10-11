// Purpose: asks the compiler Cargo runs for this project about itself.
//          Cargo builds a tiny empty crate of the adapter's own from inside
//          the project's folder, so it reads the project's settings and
//          runs the compiler it runs for the project. That compiler then
//          answers the question instead of compiling.
// Never:   works the compiler out itself from `RUSTC` or a settings file.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::ffi::OsString;
use std::path::{
    Path,
    PathBuf, //
};

/// The folder the compiler is installed in, as the compiler itself says.
pub fn home(project: &Project, supplied: &Arguments) -> Result<PathBuf, Blocked> {
    let printed = answer(project, supplied, &["--print", "sysroot"])?;
    match printed.lines().next() {
        Some(folder) => Ok(PathBuf::from(folder)),
        None => Err(Blocked::Unreadable {
            what: "rustc --print sysroot".to_string(),
            cause: "the compiler printed nothing".to_string(),
        }),
    }
}

/// What the compiler prints when Cargo runs it with `question` added. The
/// request is `cargo rustc` for the adapter's own crate, built in the
/// adapter's own folder, with the owner's settings flags.
pub fn answer(
    project: &Project,
    supplied: &Arguments,
    question: &[&str],
) -> Result<String, Blocked> {
    let target = super::cargo_target::directory(project, supplied)?;
    let folder = target.join("hyperray-finished");
    let manifest = super::cargo_compiler_crate::write(&folder)?;
    let mut arguments = vec![OsString::from("rustc"), OsString::from("--lib")];
    arguments.push(OsString::from("--manifest-path"));
    arguments.push(manifest.into_os_string());
    arguments.push(OsString::from("--target-dir"));
    arguments.push(folder.into_os_string());
    arguments.extend(super::cargo_settings::settings(&supplied.build));
    arguments.push(OsString::from("--"));
    for word in question {
        arguments.push(OsString::from(word));
    }
    let cargo = Path::new("cargo");
    let process = crate::cargo_observation::capture(cargo, &arguments, &project.root)?;
    super::cargo_build_result::stdout(&process)
}
