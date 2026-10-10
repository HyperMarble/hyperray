// Purpose: runs the native Cargo request and collects its reported files.
// Never: chooses a profile or translates the caller's Cargo arguments.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::project::Project;

pub use super::cargo_build_types::{
    BuildOutput,
    Built, //
};

/// Builds the caller's request with Cargo's lock-file and JSON requirements.
pub fn build(project: &Project, supplied: &Arguments) -> Result<BuildOutput, Blocked> {
    let members = super::cargo_members::workspace_members(project, supplied)?;
    let request = super::cargo_build_request::request(project, None, supplied);
    let process = crate::cargo_observation::observe(&request)?;
    super::cargo_build_result::finish(process, &members)
}
