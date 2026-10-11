// Purpose: asks Cargo for the finished form of each library the build made.
//          A library's own file (`.rlib`) still has places the linker has
//          not filled; `cargo rustc --crate-type dylib` makes the linked one.
//          It is built in a folder of its own, so it can never replace a
//          file the owner's build made.
// Never:   links a library itself, or hides why Cargo would not.
use crate::blocked::Blocked;
use crate::build_facts::NotBuilt;
use crate::cargo_build::{
    BuildOutput,
    Built, //
};
use crate::cargo_messages::files_of;
use crate::cargo_observation::Observation;
use crate::cargo_request::Arguments;
use crate::project::Project;
use serde_json::Value;
use std::ffi::OsString;
use std::path::Path;

/// What the record calls a finished library, as it calls nothing Cargo names.
const KIND: &str = "finished library";

/// Adds each library's finished file to `output`, built under `folder`, or
/// Cargo's words for why there is none.
pub(super) fn add(
    project: &Project,
    supplied: &Arguments,
    folder: &Path,
    output: &mut BuildOutput,
) -> Result<(), Blocked> {
    let mut libraries = output.libraries.clone();
    libraries.dedup();
    for package in libraries {
        let arguments = arguments(supplied, &package, folder);
        let cargo = Path::new("cargo");
        let process = crate::cargo_observation::capture(cargo, &arguments, &project.root)?;
        if process.success {
            output.files.extend(files(&process, &package)?);
        } else {
            let reason = super::cargo_refusal::refusal(&process);
            let kind = KIND.to_string();
            output.not_built.push(NotBuilt {
                kind,
                package,
                reason,
            });
        }
    }
    Ok(())
}

/// `cargo rustc` for one package's library as a linked dynamic library,
/// with the owner's own arguments and the lock file enforced, built under
/// `folder`.
fn arguments(supplied: &Arguments, package: &str, folder: &Path) -> Vec<OsString> {
    let words = ["rustc", "-p", package, "--lib", "--crate-type", "dylib"];
    let mut arguments: Vec<OsString> = words.into_iter().map(OsString::from).collect();
    arguments.extend(crate::cargo_request::locked(&supplied.build));
    arguments.extend(crate::cargo_request::without_message_format(
        &supplied.build,
    ));
    arguments.push(OsString::from("--target-dir"));
    arguments.push(folder.as_os_str().to_owned());
    arguments.push(OsString::from("--message-format=json"));
    arguments
}

/// True for the artifact this module asked Cargo for: the one it reports
/// with crate type `dylib`. A build script of the same package is another.
fn is_asked_for(message: &Value) -> bool {
    let types = message["target"]["crate_types"].as_array();
    let dynamic = types.is_some_and(|types| types.contains(&Value::from("dylib")));
    dynamic && message["reason"] == "compiler-artifact"
}

/// The files Cargo reports for the package's library, each marked finished.
fn files(process: &Observation, package: &str) -> Result<Vec<Built>, Blocked> {
    let printed = String::from_utf8_lossy(&process.stdout);
    let mut found = Vec::new();
    for line in printed.lines().filter(|line| line.starts_with('{')) {
        let message = super::cargo_build_result::parse(line, "cargo rustc output")?;
        if message["package_id"] == package && is_asked_for(&message) {
            found.extend(files_of(&message)?);
        }
    }
    found
        .iter_mut()
        .for_each(|file| file.kind = vec![KIND.to_string()]);
    Ok(found)
}
