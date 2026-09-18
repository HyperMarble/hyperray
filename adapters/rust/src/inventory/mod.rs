// The inventory module owns one compiler invocation and its linked artifact.
// It never accepts caller claims for tool success, output freshness, or hashes.

mod build;
mod command;
mod error;
mod identity;
mod manifest;
mod request;

pub use build::{build, BuildArtifacts};
pub use error::BuildError;
pub use manifest::{
    ArtifactIdentity, BuildManifest, SysrootFileIdentity, SysrootIdentity, ToolIdentity,
};
pub use request::{BuildRequest, ExternArtifact};
