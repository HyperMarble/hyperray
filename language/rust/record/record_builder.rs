// Purpose: assembles the files and actual request from one Cargo build.
// Never: replaces Cargo's native request with adapter-defined build choices.
use crate::blocked::Blocked;
use crate::cargo_build::Built;
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
    cargo_compiler,
    cargo_loaded_libraries,
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
    let compiler = cargo_compiler::home(&project, supplied)?;
    let libraries = cargo_loaded_libraries::loaded(&project, supplied, &built.files)?;
    let configuration = config_files::report_for_request(&project.root, &supplied.build)?;
    let mut artifacts = Vec::new();
    for file in built.files {
        artifacts.push(built_file(file)?);
    }
    for library in libraries {
        artifacts.push(compiler_library(&library)?);
    }
    Ok(BuildRecord {
        language: "rust".to_string(),
        cargo_request: built.request,
        artifacts,
        not_built: built.not_built,
        toolchain: toolchain::toolchain(&compiler, root)?,
        settings: Settings {
            lock_file: digest_of(&project.lock_file)?,
            config_files: configuration.files,
            missing_optional_config_files: configuration.missing_optional,
        },
        environment: environment::capture_environment(),
        native_code: built.native_code,
        c_toolchain: c_toolchain::c_toolchain(root)?,
        os_build: os::os_build(root)?,
    })
}

/// A file this build made, named by its content, with how Cargo compiled it.
fn built_file(file: Built) -> Result<Artifact, Blocked> {
    Ok(Artifact {
        kind: file.kind,
        file: digest_of(&file.path)?,
        features: file.features,
        compiled: Some(file.compiled),
        debug_info: file
            .debug_info
            .as_deref()
            .map(debug_info::dwarf_digest)
            .transpose()?,
    })
}

/// A library the compiler ships ready made, named by its content. This
/// build did not compile it, so it has no features and no build settings.
fn compiler_library(file: &Path) -> Result<Artifact, Blocked> {
    Ok(Artifact {
        kind: vec![cargo_loaded_libraries::KIND.to_string()],
        file: digest_of(file)?,
        features: Vec::new(),
        compiled: None,
        debug_info: None,
    })
}
