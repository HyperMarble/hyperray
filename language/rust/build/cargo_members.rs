// Purpose: asks Cargo for the workspace members used to identify project files.
// Never: parses package or workspace selection flags itself.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::project::Project;
use serde_json::Value;
use std::ffi::OsString;
use std::path::Path;

pub(super) fn workspace_members(
    project: &Project,
    supplied: &Arguments,
) -> Result<Vec<Value>, Blocked> {
    let words = ["metadata", "--no-deps", "--locked", "--format-version", "1"];
    let mut arguments: Vec<OsString> = words.into_iter().map(OsString::from).collect();
    arguments.extend(supplied.metadata.iter().cloned());
    let process = crate::cargo_observation::capture(Path::new("cargo"), &arguments, &project.root)?;
    let text = super::cargo_build_result::stdout(&process)?;
    let metadata = super::cargo_build_result::parse(&text, "cargo metadata")?;
    metadata["workspace_members"]
        .as_array()
        .cloned()
        .ok_or_else(|| Blocked::Unreadable {
            what: "cargo metadata".to_string(),
            cause: "no workspace_members".to_string(),
        })
}
