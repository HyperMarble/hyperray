// Purpose: runs the native Cargo request and collects its reported files.
// Never: chooses a profile or translates the caller's Cargo arguments.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::ffi::OsString;
use std::path::Path;

pub use super::cargo_build_types::{
    BuildOutput,
    Built, //
};

/// Builds the caller's request with Cargo's lock-file and JSON requirements,
/// then asks Cargo for the finished form of each library that build made.
pub fn build(project: &Project, supplied: &Arguments) -> Result<BuildOutput, Blocked> {
    let members = super::cargo_members::workspace_members(project, supplied)?;
    let arguments = arguments(supplied);
    let cargo = Path::new("cargo");
    let process = crate::cargo_observation::capture(cargo, &arguments, &project.root)?;
    let mut output = super::cargo_build_result::finish(process, &members)?;
    super::cargo_finished::add(project, supplied, &mut output)?;
    Ok(output)
}

/// `cargo build` with the owner's arguments, the lock file enforced, and
/// Cargo's JSON messages switched on in place of any format the owner chose.
fn arguments(supplied: &Arguments) -> Vec<OsString> {
    let mut arguments = vec![OsString::from("build")];
    arguments.extend(crate::cargo_request::locked(&supplied.build));
    arguments.extend(crate::cargo_request::without_message_format(
        &supplied.build,
    ));
    arguments.push(OsString::from("--message-format=json"));
    arguments
}
