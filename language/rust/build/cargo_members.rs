// Purpose: asks Cargo which crates are the project's own, and where it
//          puts what it builds.
// Never: parses package or workspace selection flags itself.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::project::Project;
use serde_json::Value;
use std::ffi::OsString;
use std::path::{
    Path,
    PathBuf, //
};

/// What Cargo says about the workspace the project is in.
pub(super) struct Workspace {
    pub(super) members: Vec<Value>,
    pub(super) target: PathBuf,
}

pub(super) fn workspace(project: &Project, supplied: &Arguments) -> Result<Workspace, Blocked> {
    let words = ["metadata", "--no-deps", "--format-version", "1"];
    let mut arguments: Vec<OsString> = words.into_iter().map(OsString::from).collect();
    arguments.extend(crate::cargo_request::locked(&supplied.metadata));
    arguments.extend(supplied.metadata.iter().cloned());
    let process = crate::cargo_observation::capture(Path::new("cargo"), &arguments, &project.root)?;
    let text = super::cargo_build_result::stdout(&process)?;
    let metadata = super::cargo_build_result::parse(&text, "cargo metadata")?;
    let members = metadata["workspace_members"].as_array().cloned();
    let target = metadata["target_directory"].as_str().map(PathBuf::from);
    Ok(Workspace {
        members: members.ok_or_else(|| missing("workspace_members"))?,
        target: target.ok_or_else(|| missing("target_directory"))?,
    })
}

fn missing(field: &str) -> Blocked {
    Blocked::Unreadable {
        what: "cargo metadata".to_string(),
        cause: format!("no {field}"),
    }
}
