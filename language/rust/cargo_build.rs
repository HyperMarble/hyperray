// Purpose: runs the project's own build, as the user chose it, and lists the
//          files it made and the native code linked into them.
// Never:   changes Cargo.lock, or reports a dependency's file as the project's.
use crate::blocked::Blocked;
use crate::build_facts::{Compiled, NativeCode};
use crate::cargo_messages::{files_of, native_code_of};
use crate::choice::Choice;
use crate::project::Project;
use crate::run::printed;
use serde_json::Value;
use std::path::PathBuf;

/// One file the build made, with the kind of target that made it and how.
#[derive(Debug, PartialEq)]
pub struct Built {
    pub kind: String,
    pub path: PathBuf,
    /// The features Cargo reports this file was built with.
    pub features: Vec<String>,
    pub compiled: Compiled,
    /// Apple's `.dSYM` debug-info bundle for this file, when Cargo made one.
    pub debug_info: Option<PathBuf>,
}

/// Everything one build reported: the project's own files, and the native
/// code any package's build script linked in (dependencies' too).
#[derive(Debug, PartialEq)]
pub struct BuildOutput {
    pub files: Vec<Built>,
    pub native_code: Vec<NativeCode>,
}

/// Builds what `choice` asks for, with `--locked`, so Cargo fails rather
/// than change pinned versions.
pub fn build(project: &Project, choice: &Choice) -> Result<BuildOutput, Blocked> {
    let members = workspace_members(project)?;
    let mut args = vec!["build".to_string(), "--locked".to_string()];
    args.extend(choice.cargo_args());
    args.push("--message-format=json".to_string());
    let args: Vec<&str> = args.iter().map(String::as_str).collect();
    let messages = printed("cargo", &args, &project.root)?;
    let mut output = BuildOutput {
        files: Vec::new(),
        native_code: Vec::new(),
    };
    // Cargo's messages are its JSON lines; any other line is the compiler
    // printing what the project's own settings asked for, not a message.
    for line in messages.lines().filter(|line| line.starts_with('{')) {
        let message = parse(line, "cargo build output")?;
        if is_member_artifact(&message, &members) {
            output.files.extend(files_of(&message)?);
        }
        if message["reason"] == "build-script-executed" {
            output.native_code.extend(native_code_of(&message));
        }
    }
    if output.files.is_empty() {
        return Err(Blocked::NoArtifact);
    }
    Ok(output)
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
