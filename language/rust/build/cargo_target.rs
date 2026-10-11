// Purpose: asks Cargo where it puts what it builds for this project.
// Never: works that folder out itself from flags or settings.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::project::Project;
use std::ffi::OsString;
use std::path::{
    Path,
    PathBuf, //
};

pub(super) fn directory(project: &Project, supplied: &Arguments) -> Result<PathBuf, Blocked> {
    let words = ["metadata", "--no-deps", "--format-version", "1"];
    let mut arguments: Vec<OsString> = words.into_iter().map(OsString::from).collect();
    arguments.extend(crate::cargo_request::locked(&supplied.metadata));
    arguments.extend(supplied.metadata.iter().cloned());
    let process = crate::cargo_observation::capture(Path::new("cargo"), &arguments, &project.root)?;
    let text = super::cargo_build_result::stdout(&process)?;
    let metadata = super::cargo_build_result::parse(&text, "cargo metadata")?;
    let target = metadata["target_directory"].as_str().map(PathBuf::from);
    target.ok_or_else(|| Blocked::Unreadable {
        what: "cargo metadata".to_string(),
        cause: "no target_directory".to_string(),
    })
}
