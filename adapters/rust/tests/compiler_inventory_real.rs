// This test calls the public build owner with the pinned compiler driver.
// It accepts only artifacts created by the compiler and linker processes.

use hyperray_rust::inventory::{build, BuildRequest, ExternArtifact};
use hyperray_rust::mir::RawInventory;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

fn fixture_source() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../fixtures/rust/compiler-inventory/src/lib.rs")
}

fn dependency_source() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../fixtures/rust/compiler-inventory-dependency/src/lib.rs")
}

fn output_directory() -> PathBuf {
    PathBuf::from("/Users/hak").join(format!("hyperray-rust-real-build-{}", std::process::id()))
}

fn dependency_artifact(
    directory: &Path,
    driver: &Path,
    sysroot: &Path,
) -> Result<PathBuf, Box<dyn std::error::Error>> {
    let artifact = directory.join("libcompiler_inventory_dependency_alias.rlib");
    let status = Command::new(driver)
        .current_dir(directory)
        .env_remove("HYPERRAY_INVENTORY_DIR")
        .env_remove("CARGO_PRIMARY_PACKAGE")
        .args([
            "--crate-name",
            "compiler_inventory_dependency_alias",
            "--crate-type=rlib",
            "--edition=2021",
            "--target",
            "aarch64-apple-darwin",
            "--emit",
            &format!("link={}", artifact.display()),
        ])
        .arg("--sysroot")
        .arg(sysroot)
        .arg(dependency_source())
        .status()?;
    if !status.success() {
        return Err("dependency compiler invocation failed".into());
    }
    Ok(artifact)
}

fn pinned_sysroot() -> PathBuf {
    PathBuf::from("/Users/hak/.rustup/toolchains/nightly-2026-08-21-aarch64-apple-darwin")
}

fn remove_directory(path: &Path) -> Result<(), Box<dyn std::error::Error>> {
    match fs::remove_dir_all(path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(Box::new(error)),
    }
}

#[test]
fn public_build_owner_records_real_compiler_inventory() -> Result<(), Box<dyn std::error::Error>> {
    let driver = std::env::var("HYPERRAY_MIR_DUMP")
        .map(PathBuf::from)
        .map_err(|error| format!("HYPERRAY_MIR_DUMP is required: {error}"))?;
    let output = output_directory();
    remove_directory(&output)?;
    let dependency_directory = PathBuf::from(format!("{}.dependency", output.display()));
    remove_directory(&dependency_directory)?;
    fs::create_dir(&dependency_directory)?;
    let sysroot = pinned_sysroot();
    let dependency = dependency_artifact(&dependency_directory, &driver, &sysroot)?;
    let request = BuildRequest {
        driver,
        linker: PathBuf::from("/usr/bin/clang"),
        source: fixture_source(),
        output_directory: output.clone(),
        target: "aarch64-apple-darwin".to_string(),
        compiler_flags: vec![
            "--crate-type=lib".to_string(),
            "--edition=2021".to_string(),
            "--cfg".to_string(),
            "hyperray_thread_local".to_string(),
            "-C".to_string(),
            "codegen-units=4".to_string(),
        ],
        linker_flags: vec!["-r".to_string()],
        boundary_artifact: None,
        extern_artifacts: vec![ExternArtifact {
            name: "compiler_inventory_dependency_alias".to_string(),
            path: dependency.clone(),
        }],
        sysroot: Some(sysroot.clone()),
    };
    let result = build(&request)?;
    let bytes = fs::read(&result.inventory_path)?;
    let inventory: RawInventory = serde_json::from_slice(&bytes)?;
    assert!(!inventory.instances.is_empty());
    let names: Vec<&str> = inventory
        .instances
        .iter()
        .map(|instance| instance.name.as_str())
        .collect();
    assert!(inventory
        .instances
        .iter()
        .any(|instance| instance.kind == "static" && instance.name.contains("RETAINED_VALUE")));
    let shim = inventory
        .instances
        .iter()
        .find(|instance| instance.kind == "shim")
        .ok_or("compiler shim instance is missing")?;
    assert!(shim.body.is_none() || shim.body.as_ref().is_some_and(|body| body.phase == "shim"));
    assert!(names.iter().any(|name| name.contains("repeated::<u8>")));
    assert!(names.iter().any(|name| name.contains("repeated::<u64>")));
    let cleanup = inventory
        .instances
        .iter()
        .find(|instance| instance.name.contains("cleanup_case"))
        .ok_or("cleanup_case instance is missing")?;
    let cleanup_body = cleanup
        .body
        .as_ref()
        .ok_or("cleanup_case body is missing")?;
    let cleanup_payload = serde_json::to_string(&cleanup_body.payload)?;
    assert!(cleanup_payload.contains("Drop"));
    assert!(cleanup_payload.contains("source_scopes_unexposed_by_public_api"));
    let thread_local = inventory
        .instances
        .iter()
        .find(|instance| instance.name.contains("THREAD_LOCAL_VALUE"))
        .ok_or("thread-local static instance is missing")?;
    assert_eq!(thread_local.kind, "static");
    let repeated_array = inventory
        .instances
        .iter()
        .find(|instance| instance.name.contains("repeated_array"))
        .ok_or("repeated_array instance is missing")?;
    let body = repeated_array
        .body
        .as_ref()
        .ok_or("repeated_array body is missing")?;
    let payload = serde_json::to_string(&body.payload)?;
    assert!(payload.contains("\"blocks\""));
    assert!(payload.contains("\"statements\""));
    assert!(payload.contains("\"terminator\""));
    assert!(payload.contains("Repeat"));
    assert!(payload.contains("\"locals\""));
    assert!(payload.contains("\"arg_count\""));
    assert!(payload.contains("\"spread_arg\""));
    assert!(payload.contains("\"source_scopes\""));
    assert!(result
        .manifest
        .compiler_arguments
        .iter()
        .any(|argument| { argument == "codegen-units=4" }));
    let expected_sysroot = sysroot.display().to_string();
    assert!(result
        .manifest
        .compiler_arguments
        .windows(2)
        .any(|arguments| { arguments[0] == "--sysroot" && arguments[1] == expected_sysroot }));
    let manifest_sysroot = result
        .manifest
        .sysroot
        .as_ref()
        .ok_or("sysroot evidence is missing")?;
    assert_eq!(manifest_sysroot.target, "aarch64-apple-darwin");
    assert!(!manifest_sysroot.files.is_empty());
    assert!(manifest_sysroot
        .files
        .windows(2)
        .all(|files| files[0].path < files[1].path));
    assert_eq!(manifest_sysroot.aggregate_sha256.len(), 64);
    assert_unique_instance_ids(&inventory);
    assert!(result.manifest.elf.size > 0);
    remove_directory(&output)?;
    remove_directory(&dependency_directory)?;
    Ok(())
}

fn assert_unique_instance_ids(inventory: &RawInventory) {
    let mut ids: Vec<&str> = inventory
        .instances
        .iter()
        .map(|instance| instance.id.as_str())
        .collect();
    ids.sort_unstable();
    ids.dedup();
    assert_eq!(ids.len(), inventory.instances.len());
}
