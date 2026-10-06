// Purpose: the Rust adapter: build a project its own way, record what came out.
// Never:   read instructions or prove anything; the loader and engine do that.
pub mod blocked;
pub mod cargo_build;
pub mod digest;
pub mod os;
pub mod project;
pub mod record;
pub mod run;
pub mod toolchain;

use blocked::Blocked;
use digest::digest_of;
use record::{Artifact, BuildRecord, Outcome, Settings};
use std::path::Path;

/// Builds the Cargo project at `root` and returns its build record, or the
/// reason it could not be built.
pub fn build_record(root: &Path) -> Outcome {
    match try_build_record(root) {
        Ok(record) => Outcome::Built(Box::new(record)),
        Err(blocked) => Outcome::Blocked {
            reason: blocked.to_string(),
        },
    }
}

fn try_build_record(root: &Path) -> Result<BuildRecord, Blocked> {
    let project = project::find(root)?;
    let built = cargo_build::build(&project)?;
    let mut artifacts = Vec::new();
    for file in built {
        artifacts.push(Artifact {
            kind: file.kind,
            file: digest_of(&file.path)?,
        });
    }
    Ok(BuildRecord {
        language: "rust".to_string(),
        artifacts,
        toolchain: toolchain::toolchain(root)?,
        settings: Settings {
            profile: "release".to_string(),
            lock_file: digest_of(&project.lock_file)?,
        },
        os_build: os::os_build(root)?,
    })
}
