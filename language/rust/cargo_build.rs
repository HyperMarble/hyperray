// Purpose: runs the project's own release build and lists the files it made.
// Never:   changes Cargo.lock, or reports a dependency's file as the project's.
use crate::blocked::Blocked;
use crate::project::Project;
use crate::run::printed;
use serde_json::Value;
use std::path::PathBuf;

/// One file the build made, with the kind of target that made it.
#[derive(Debug, PartialEq)]
pub struct Built {
    pub kind: String,
    pub path: PathBuf,
}

/// Builds with `--locked`, so Cargo fails rather than change pinned versions.
pub fn build(project: &Project) -> Result<Vec<Built>, Blocked> {
    let members = workspace_members(project)?;
    let args = ["build", "--release", "--locked", "--message-format=json"];
    let messages = printed("cargo", &args, &project.root)?;
    let mut found = Vec::new();
    for line in messages.lines() {
        let message = parse(line, "cargo build output")?;
        if is_member_artifact(&message, &members) {
            found.extend(files_of(&message));
        }
    }
    if found.is_empty() {
        return Err(Blocked::NoArtifact);
    }
    Ok(found)
}

/// The package ids of the project's own crates, not its dependencies.
fn workspace_members(project: &Project) -> Result<Vec<Value>, Blocked> {
    let args = ["metadata", "--no-deps", "--locked", "--format-version", "1"];
    let metadata = parse(&printed("cargo", &args, &project.root)?, "cargo metadata")?;
    let members = metadata["workspace_members"].as_array().cloned();
    members.ok_or_else(|| Blocked::Unreadable {
        what: "cargo metadata".to_string(),
        cause: "no workspace_members".to_string(),
    })
}

fn parse(text: &str, what: &str) -> Result<Value, Blocked> {
    serde_json::from_str(text).map_err(|error| Blocked::Unreadable {
        what: what.to_string(),
        cause: error.to_string(),
    })
}

fn is_member_artifact(message: &Value, members: &[Value]) -> bool {
    message["reason"] == "compiler-artifact" && members.contains(&message["package_id"])
}

/// The compiled files of one artifact message. Metadata-only `.rmeta`
/// files hold no machine code, so they are not part of the build record.
fn files_of(message: &Value) -> Vec<Built> {
    let kind = message["target"]["kind"][0].as_str().unwrap_or("unknown");
    let files = message["filenames"].as_array().cloned().unwrap_or_default();
    files
        .iter()
        .filter_map(Value::as_str)
        .filter(|path| !path.ends_with(".rmeta"))
        .map(|path| Built {
            kind: kind.to_string(),
            path: PathBuf::from(path),
        })
        .collect()
}
