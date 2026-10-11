// Purpose: a folder is a project only with Cargo.toml and a Cargo.lock, its
//          own or, for a workspace member, the one at the workspace's top.
// Never:   accepts a folder whose dependency versions are not pinned.
mod sample_project;
mod sample_workspace;

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

#[test]
fn a_workspace_member_is_found_with_the_workspace_lock_file() -> Result<(), String> {
    let (top, member) = sample_workspace::create("locked_workspace", true)?;
    let project = find(&member).map_err(|blocked| blocked.to_string())?;
    assert_eq!(project.root, member);
    assert_eq!(project.lock_file, top.join("Cargo.lock"));
    Ok(())
}

#[test]
fn a_workspace_member_without_any_lock_file_is_blocked() -> Result<(), String> {
    let (top, member) = sample_workspace::create("unlocked_workspace", false)?;
    assert_eq!(find(&member), Err(Blocked::NoLockFile(top)));
    Ok(())
}
