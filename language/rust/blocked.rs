// Purpose: every reason the adapter can give for not producing a build.
// Never:   hides a reason; each one reaches the user as readable text.
use std::fmt;
use std::path::PathBuf;

/// Why there is no build record.
#[derive(Debug, PartialEq)]
pub enum Blocked {
    /// The folder is not a Cargo project, so its settings are unknown.
    NoManifest(PathBuf),
    /// The project has never pinned its dependency versions.
    NoLockFile(PathBuf),
    /// A tool the build needs could not be started.
    ToolMissing { tool: String, cause: String },
    /// A tool ran and reported failure.
    ToolFailed { tool: String, output: String },
    /// A tool printed something this adapter cannot read.
    Unreadable { what: String, cause: String },
    /// The build finished but produced none of the project's own files.
    NoArtifact,
}

impl fmt::Display for Blocked {
    fn fmt(&self, out: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NoManifest(root) => write!(
                out,
                "{} has no Cargo.toml: which Rust version and settings should be used?",
                root.display()
            ),
            Self::NoLockFile(root) => write!(
                out,
                "{} has no Cargo.lock: run the project's build once to pin its versions",
                root.display()
            ),
            Self::ToolMissing { tool, cause } => write!(out, "{tool} could not start: {cause}"),
            Self::ToolFailed { tool, output } => write!(out, "{tool} failed: {output}"),
            Self::Unreadable { what, cause } => write!(out, "cannot read {what}: {cause}"),
            Self::NoArtifact => write!(out, "the build produced no files from this project"),
        }
    }
}
