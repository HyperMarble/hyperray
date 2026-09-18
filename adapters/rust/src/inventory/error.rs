// BuildError preserves the failing process and artifact boundary.
// A failed build never becomes a successful BuildManifest.

use std::fmt::{Display, Formatter};
use std::path::PathBuf;

#[derive(Debug)]
pub enum BuildError {
    InvalidRequest(String),
    OutputDirectoryExists(PathBuf),
    Io(String),
    CompilerFailed { status: Option<i32>, log: PathBuf },
    LinkerFailed { status: Option<i32>, log: PathBuf },
    MissingArtifact(PathBuf),
    HashFailed(PathBuf, String),
    InputChanged(PathBuf),
    Manifest(String),
}

impl Display for BuildError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidRequest(reason) => write!(formatter, "invalid build request: {reason}"),
            Self::OutputDirectoryExists(path) => {
                write!(
                    formatter,
                    "output directory already exists: {}",
                    path.display()
                )
            }
            Self::Io(reason) => write!(formatter, "build I/O failed: {reason}"),
            Self::CompilerFailed { status, log } => {
                write!(
                    formatter,
                    "compiler failed with {status:?}; see {}",
                    log.display()
                )
            }
            Self::LinkerFailed { status, log } => {
                write!(
                    formatter,
                    "linker failed with {status:?}; see {}",
                    log.display()
                )
            }
            Self::MissingArtifact(path) => {
                write!(formatter, "missing build artifact: {}", path.display())
            }
            Self::HashFailed(path, reason) => {
                write!(formatter, "cannot hash {}: {reason}", path.display())
            }
            Self::InputChanged(path) => {
                write!(formatter, "input changed during build: {}", path.display())
            }
            Self::Manifest(reason) => write!(formatter, "cannot write build manifest: {reason}"),
        }
    }
}

impl std::error::Error for BuildError {}
