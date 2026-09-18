use super::inputs::{self, ObservedInputs};
use super::paths::Paths;
use crate::inventory::{BuildError, BuildManifest, BuildRequest};
use serde::Serialize;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Serialize)]
pub struct BuildArtifacts {
    pub manifest: BuildManifest,
    pub manifest_path: PathBuf,
    pub object_path: PathBuf,
    pub dep_info_path: PathBuf,
    pub inventory_path: PathBuf,
    pub elf_path: PathBuf,
}

pub(super) fn manifest(
    request: &BuildRequest,
    paths: &Paths,
    inputs: &ObservedInputs,
    compilation_id: String,
    compiler_arguments: Vec<String>,
    linker_arguments: Vec<String>,
) -> Result<BuildManifest, BuildError> {
    let mut unresolved_obligations = vec![
        "dep-info does not establish complete source or runtime closure".to_string(),
        "optimized MIR does not prove pre-optimization preservation".to_string(),
        "compiler roots and semantic mappings remain unresolved".to_string(),
    ];
    if inputs.sysroot.is_none() {
        unresolved_obligations.push("selected sysroot input set is unavailable".to_string());
    } else {
        unresolved_obligations.push(
            "sysroot identity covers regular files; symlink and runtime closure remain unresolved"
                .to_string(),
        );
    }
    Ok(BuildManifest {
        version: crate::inventory::manifest::MANIFEST_VERSION,
        compilation_id,
        target: request.target.clone(),
        driver: inputs.driver.clone(),
        linker: inputs.linker.clone(),
        source: inputs.source.clone(),
        sysroot: inputs.sysroot.clone(),
        boundary_artifact: inputs.boundary.clone(),
        extern_artifacts: inputs.externs.clone(),
        object: inputs::artifact(&paths.object)?,
        dep_info: inputs::artifact(&paths.dep_info)?,
        inventory: inputs::artifact(&paths.inventory)?,
        elf: inputs::artifact(&paths.elf)?,
        compiler_arguments,
        linker_arguments,
        unresolved_obligations,
    })
}
pub(super) fn write(
    request: &BuildRequest,
    manifest: BuildManifest,
) -> Result<BuildArtifacts, BuildError> {
    let manifest_path = request.output_directory.join("manifest.json");
    let bytes = serde_json::to_vec_pretty(&manifest)
        .map_err(|error| BuildError::Manifest(error.to_string()))?;
    fs::write(&manifest_path, bytes).map_err(|error| BuildError::Manifest(error.to_string()))?;
    let paths = Paths::new(&request.output_directory);
    Ok(BuildArtifacts {
        manifest,
        manifest_path,
        object_path: paths.object,
        dep_info_path: paths.dep_info,
        inventory_path: paths.inventory,
        elf_path: paths.elf,
    })
}
