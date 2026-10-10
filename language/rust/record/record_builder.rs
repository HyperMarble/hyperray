// Purpose: assembles the files and actual request from one Cargo build.
// Never: replaces Cargo's native request with adapter-defined build choices.
use crate::blocked::Blocked;
use crate::cargo_request::Arguments;
use crate::digest::digest_of;
use crate::record::{
    Artifact,
    BuildRecord,
    Settings, //
};
use crate::{
    c_toolchain,
    cargo_build,
    config_files,
    debug_info,
    environment,
    os,
    project,
    toolchain, //
};
use std::path::Path;

pub(crate) fn build(root: &Path, supplied: &Arguments) -> Result<BuildRecord, Blocked> {
    let project = project::find(root)?;
    let built = cargo_build::build(&project, supplied)?;
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
        cargo_request: built.request,
        artifacts,
        toolchain: toolchain::toolchain(root)?,
        settings: Settings {
            lock_file: digest_of(&project.lock_file)?,
            config_files: config_files::config_files(&project.root)?,
        },
        environment: environment::build_environment(std::env::vars()),
        native_code: built.native_code,
        c_toolchain: c_toolchain::c_toolchain(root)?,
        os_build: os::os_build(root)?,
    })
}
