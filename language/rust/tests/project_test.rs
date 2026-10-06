// Purpose: a folder is a project only with both Cargo.toml and Cargo.lock.
// Never:   accepts a folder whose dependency versions are not pinned.
mod sample_project;

use language_rust::blocked::Blocked;
use language_rust::project::find;
use sample_project::{create, Sample};

#[test]
fn a_folder_without_cargo_toml_is_blocked() -> Result<(), String> {
    let root = std::path::PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join("not_a_project");
    std::fs::create_dir_all(&root).map_err(|error| error.to_string())?;
    assert_eq!(find(&root), Err(Blocked::NoManifest(root.clone())));
    Ok(())
}

#[test]
fn a_project_without_a_lock_file_is_blocked() -> Result<(), String> {
    let root = create(&Sample {
        name: "unlocked_project",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: false,
    })?;
    assert_eq!(find(&root), Err(Blocked::NoLockFile(root.clone())));
    Ok(())
}

#[test]
fn a_locked_project_is_found_with_its_lock_file() -> Result<(), String> {
    let root = create(&Sample {
        name: "locked_project",
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let project = find(&root).map_err(|blocked| blocked.to_string())?;
    assert_eq!(project.lock_file, root.join("Cargo.lock"));
    Ok(())
}
