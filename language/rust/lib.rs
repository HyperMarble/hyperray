// Purpose: the Rust adapter: build a project its own way, record what came out.
// Never:   read instructions or prove anything; the loader and engine do that.
pub mod blocked;
pub mod build_facts;
pub mod c_toolchain;
pub mod cargo_build;
pub mod cargo_messages;
pub mod choice;
pub mod config_files;
pub mod debug_info;
pub mod digest;
pub mod environment;
pub mod os;
pub mod project;
pub mod record;
pub mod run;
pub mod toolchain;

use blocked::Blocked;
use choice::Choice;
use digest::digest_of;
use record::{Artifact, BuildRecord, Outcome, Settings};
use std::path::Path;

/// Builds the Cargo project at `root` the project's own default way (release
/// profile, default features) and returns its build record, or the reason it
/// could not be built.
pub fn build_record(root: &Path) -> Outcome {
    build_record_with(root, &Choice::default())
}

/// Builds the Cargo project at `root` the way `choice` asks (a profile and a
/// feature setting) and returns its build record, or the reason it could
/// not be built.
pub fn build_record_with(root: &Path, choice: &Choice) -> Outcome {
    match try_build_record(root, choice) {
        Ok(record) => Outcome::Built(Box::new(record)),
        Err(blocked) => Outcome::Blocked {
            reason: blocked.to_string(),
        },
    }
}

fn try_build_record(root: &Path, choice: &Choice) -> Result<BuildRecord, Blocked> {
    let project = project::find(root)?;
    let built = cargo_build::build(&project, choice)?;
    let mut artifacts = Vec::new();
    for file in built.files {
        artifacts.push(Artifact {
            kind: file.kind,
            file: digest_of(&file.path)?,
            features: file.features,
            compiled: file.compiled,
            debug_info: file
                .debug_info
                .as_deref()
                .map(debug_info::dwarf_digest)
                .transpose()?,
        });
    }
    Ok(BuildRecord {
        language: "rust".to_string(),
        artifacts,
        toolchain: toolchain::toolchain(root)?,
        settings: Settings {
            requested: choice.clone(),
            lock_file: digest_of(&project.lock_file)?,
            config_files: config_files::config_files(&project.root)?,
        },
        environment: environment::build_environment(std::env::vars()),
        native_code: built.native_code,
        c_toolchain: c_toolchain::c_toolchain(root)?,
        os_build: os::os_build(root)?,
    })
}
