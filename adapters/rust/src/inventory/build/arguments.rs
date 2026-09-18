// Construct owned compiler and linker arguments without accepting overrides.

use super::paths::Paths;
use crate::inventory::BuildRequest;

pub(super) fn compiler(request: &BuildRequest, paths: &Paths) -> Vec<String> {
    let mut arguments = vec![
        request.source.display().to_string(),
        "--target".to_string(),
        request.target.clone(),
        "--emit".to_string(),
        format!(
            "obj={},dep-info={}",
            paths.object.display(),
            paths.dep_info.display()
        ),
    ];
    arguments.extend(request.compiler_flags.clone());
    add_sysroot(&mut arguments, request);
    add_externs(&mut arguments, request);
    arguments
}

fn add_sysroot(arguments: &mut Vec<String>, request: &BuildRequest) {
    if let Some(sysroot) = &request.sysroot {
        arguments.extend(["--sysroot".to_string(), sysroot.display().to_string()]);
    }
}

fn add_externs(arguments: &mut Vec<String>, request: &BuildRequest) {
    for artifact in &request.extern_artifacts {
        arguments.extend([
            "--extern".to_string(),
            format!("{}={}", artifact.name, artifact.path.display()),
        ]);
    }
}

pub(super) fn linker(request: &BuildRequest, paths: &Paths) -> Vec<String> {
    let mut arguments = vec![
        paths.object.display().to_string(),
        "-o".to_string(),
        paths.elf.display().to_string(),
    ];
    arguments.extend(
        request
            .extern_artifacts
            .iter()
            .map(|artifact| artifact.path.display().to_string()),
    );
    arguments.extend(request.linker_flags.clone());
    arguments
}
